// Package service tiene la logica de negocio de inscripciones que cruza
// servicios: antes de guardar un alta pregunta por gRPC a alumnos y materias.
package service

import (
	"context"
	"log"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"inscripciones/clients"
	"inscripciones/models"

	alumnospb "horarios/pb/alumnos"
	materiaspb "horarios/pb/materias"
)

type InscripcionService struct {
	Model   *models.InscripcionModel
	Clients *clients.Clients
	Timeout time.Duration
	Caller  string
}

func New(model *models.InscripcionModel, c *clients.Clients) *InscripcionService {
	caller := os.Getenv("INSTANCE_ID")
	if caller == "" {
		caller = "inscripciones"
	}
	return &InscripcionService{Model: model, Clients: c, Timeout: 3 * time.Second, Caller: caller}
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
//  4. guarda en la base (duplicado, cupo)
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
	_, err = s.Clients.Materias.GetMateria(c2, &materiaspb.GetMateriaRequest{Id: int32(materiaID)}, grpc.Header(&hdr))
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

	return s.Model.Create(alumnoID, materiaID)
}
