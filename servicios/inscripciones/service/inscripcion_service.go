// Package service tiene la logica de negocio de inscripciones que cruza
// servicios: antes de guardar un alta pregunta por gRPC a alumnos y materias,
// y despues de guardar (alta o baja) publica un evento en Kafka.
package service

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"inscripciones/clients"
	"inscripciones/models"

	"horarios/mensajeria"
	alumnospb "horarios/pb/alumnos"
	materiaspb "horarios/pb/materias"
)

type InscripcionService struct {
	Model     *models.InscripcionModel
	Clients   *clients.Clients
	Productor *mensajeria.Productor
	Timeout   time.Duration
	Caller    string
}

func New(model *models.InscripcionModel, c *clients.Clients, p *mensajeria.Productor) *InscripcionService {
	caller := os.Getenv("INSTANCE_ID")
	if caller == "" {
		caller = "inscripciones"
	}
	return &InscripcionService{Model: model, Clients: c, Productor: p, Timeout: 3 * time.Second, Caller: caller}
}

// llamada prepara el contexto de una llamada saliente: timeout y el header
// x-caller para que quede en los logs de quien responde.
func (s *InscripcionService) llamada(ctx context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(ctx, s.Timeout)
	return metadata.AppendToOutgoingContext(ctx, "x-caller", s.Caller), cancel
}

// traducir pasa un error gRPC del otro servicio a un error del dominio.
func traducir(err error, noExiste error) error {
	switch status.Code(err) {
	case codes.NotFound:
		return noExiste
	case codes.Unavailable, codes.DeadlineExceeded:
		return models.ErrServicioNoDisponible
	default:
		return err
	}
}

func replica(hdr metadata.MD) string {
	if v := hdr.Get("x-instance-id"); len(v) > 0 {
		return v[0]
	}
	return "?"
}

// Inscribir da de alta una materia:
//
//  1. alumnos.GetAlumno                      -> existe y de que carrera es
//  2. materias.GetMateria                    -> existe
//  3. materias.MateriaPerteneceACarrera      -> es de su carrera (si no, 403)
//  4. revisa que no se empalme con lo que el alumno ya tiene inscrito
//  5. guarda en la base (duplicado, cupo)
//  6. publica inscripcion.creada en Kafka (asincrono, no bloquea la respuesta)
func (s *InscripcionService) Inscribir(ctx context.Context, alumnoID, materiaID int) (*models.Inscripcion, error) {
	var hdr metadata.MD

	c1, cancel := s.llamada(ctx)
	alumno, err := s.Clients.Alumnos.GetAlumno(c1, &alumnospb.GetAlumnoRequest{Id: int32(alumnoID)}, grpc.Header(&hdr))
	cancel()
	if err != nil {
		log.Printf("inscripciones -> alumnos.GetAlumno(%d): %v", alumnoID, status.Code(err))
		return nil, traducir(err, models.ErrAlumnoNoExiste)
	}
	log.Printf("inscripciones -> alumnos.GetAlumno(%d) ok (atendio %s)", alumnoID, replica(hdr))

	c2, cancel := s.llamada(ctx)
	materia, err := s.Clients.Materias.GetMateria(c2, &materiaspb.GetMateriaRequest{Id: int32(materiaID)}, grpc.Header(&hdr))
	cancel()
	if err != nil {
		log.Printf("inscripciones -> materias.GetMateria(%d): %v", materiaID, status.Code(err))
		return nil, traducir(err, models.ErrMateriaNoExiste)
	}
	log.Printf("inscripciones -> materias.GetMateria(%d) ok (atendio %s)", materiaID, replica(hdr))

	c3, cancel := s.llamada(ctx)
	res, err := s.Clients.Materias.MateriaPerteneceACarrera(c3,
		&materiaspb.PerteneceRequest{MateriaId: int32(materiaID), CarreraId: alumno.GetCarreraId()}, grpc.Header(&hdr))
	cancel()
	if err != nil {
		log.Printf("inscripciones -> materias.MateriaPerteneceACarrera: %v", status.Code(err))
		return nil, traducir(err, models.ErrMateriaNoExiste)
	}
	log.Printf("inscripciones -> materias.MateriaPerteneceACarrera(%d,%d)=%v (atendio %s)",
		materiaID, alumno.GetCarreraId(), res.GetPertenece(), replica(hdr))
	if !res.GetPertenece() {
		return nil, models.ErrOtraCarrera
	}

	// 4. Empalme de horario contra lo que el alumno ya tiene inscrito.
	actuales, err := s.Model.GetByAlumnoID(alumnoID)
	if err != nil {
		return nil, err
	}
	for _, a := range actuales {
		if a.Estado == "cursada" || a.MateriaID == materiaID {
			continue // lo ya cursado no ocupa horario; la misma materia la rechaza Create (ya inscrito)
		}
		if Empalman(materia.GetHorario(), materia.GetDias(), a.Horario, a.Dias) {
			log.Printf("inscripciones: alta de %s rechazada, empalma con %s", materia.GetClave(), a.Clave)
			return nil, fmt.Errorf("%w: %s - %s (%s %s)", models.ErrEmpalme, a.Clave, a.Materia, a.Dias, a.Horario)
		}
	}

	item, err := s.Model.Create(alumnoID, materiaID)
	if err != nil {
		return nil, err
	}

	s.Productor.Publicar(mensajeria.TopicInscripciones, strconv.Itoa(materiaID), mensajeria.TipoInscripcionCreada,
		mensajeria.InscripcionEvento{
			InscripcionID: item.ID,
			AlumnoID:      alumnoID,
			AlumnoNombre:  alumno.GetNombre(),
			AlumnoCorreo:  alumno.GetCorreo(),
			MateriaID:     materiaID,
			MateriaClave:  materia.GetClave(),
			MateriaNombre: materia.GetNombre(),
		})
	return item, nil
}

// DarDeBaja borra una inscripcion y publica inscripcion.cancelada en Kafka.
// Devuelve sql.ErrNoRows si no existe y models.ErrYaCursada si ya esta cursada.
func (s *InscripcionService) DarDeBaja(ctx context.Context, id int) error {
	insc, err := s.Model.GetByID(id)
	if err != nil {
		return err
	}
	if err := s.Model.Delete(id); err != nil {
		return err
	}

	ev := mensajeria.InscripcionEvento{
		InscripcionID: id,
		AlumnoID:      insc.AlumnoID,
		MateriaID:     insc.MateriaID,
		MateriaClave:  insc.Clave,
		MateriaNombre: insc.Materia,
	}
	// Enriquecer con el alumno es "lo mejor posible": si alumnos no responde,
	// la baja ya esta hecha y el evento sale solo con los ids.
	c, cancel := s.llamada(ctx)
	defer cancel()
	if al, err := s.Clients.Alumnos.GetAlumno(c, &alumnospb.GetAlumnoRequest{Id: int32(insc.AlumnoID)}); err == nil {
		ev.AlumnoNombre = al.GetNombre()
		ev.AlumnoCorreo = al.GetCorreo()
	}
	s.Productor.Publicar(mensajeria.TopicInscripciones, strconv.Itoa(insc.MateriaID), mensajeria.TipoInscripcionCancelada, ev)
	return nil
}
