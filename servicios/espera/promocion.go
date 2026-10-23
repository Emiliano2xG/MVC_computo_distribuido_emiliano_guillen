package main

import (
	"context"
	"log"
	"strconv"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"horarios/mensajeria"
	inscpb "horarios/pb/inscripciones"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var resueltas = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "espera_resueltas_total",
	Help: "Lugares de la lista de espera cerrados, por resultado (inscrito / descartada / cancelada).",
}, []string{"resultado"})

// Promotor reacciona a materia.cupo_liberado: cada vez que se libera un lugar
// inscribe al primero de la fila llamando por gRPC a inscripciones (la misma
// validacion que una alta normal: carrera, cupo, empalme de horario...).
type Promotor struct {
	Store     *Store
	Clients   *Clients
	Productor *mensajeria.Productor
}

// Manejar es el Handler del consumidor de Kafka (grupo "espera").
func (p *Promotor) Manejar(ctx context.Context, topico string, ev mensajeria.Evento) error {
	if ev.Tipo != mensajeria.TipoMateriaCupoLiberado {
		return nil // los demas eventos de materias no me interesan
	}
	var d mensajeria.MateriaCupo
	if err := ev.Decodificar(&d); err != nil {
		return nil // mal formado: reintentar no sirve
	}

	// Puede haberse liberado mas de un lugar antes de que llegara este evento,
	// asi que se inscribe gente mientras haya cupo: cuando se llena, inscripciones
	// contesta "sin cupo" y se corta el ciclo.
	for {
		sig, err := p.Store.Siguiente(ctx, d.MateriaID)
		if err != nil {
			return err // se reintenta
		}
		if sig == nil {
			return nil // nadie esperando
		}

		c, cancel := p.Clients.Llamada(ctx)
		_, err = p.Clients.Inscripciones.Inscribir(c, &inscpb.InscribirRequest{
			AlumnoId: int32(sig.AlumnoID), MateriaId: int32(sig.MateriaID),
		})
		cancel()

		if err == nil {
			log.Printf("espera: se libero %s y se inscribio a alumno %d (lugar %d de la fila)", sig.MateriaClave, sig.AlumnoID, sig.ID)
			if ok, err := p.Store.Resolver(ctx, sig.ID, "inscrito", ""); err != nil {
				return err
			} else if ok {
				resueltas.WithLabelValues("inscrito").Inc()
				p.publicar(mensajeria.TipoEsperaPromovida, *sig, "")
			}
			continue
		}

		msg := status.Convert(err).Message()
		switch status.Code(err) {
		case codes.Unavailable, codes.DeadlineExceeded:
			return err // inscripciones no responde: el consumidor reintenta
		case codes.FailedPrecondition:
			if strings.Contains(msg, "cupo") {
				log.Printf("espera: %s se volvio a llenar, la fila sigue", sig.MateriaClave)
				return nil // alguien mas tomo el lugar; esperamos el siguiente evento
			}
			// ya la cursada: no tiene caso que siga en la fila
			if err := p.descartar(ctx, *sig, msg); err != nil {
				return err
			}
		case codes.AlreadyExists:
			// Ya esta inscrito (por ejemplo, un reintento tras confirmar): el objetivo se cumplio.
			if _, err := p.Store.Resolver(ctx, sig.ID, "inscrito", "ya estaba inscrito"); err != nil {
				return err
			}
			resueltas.WithLabelValues("inscrito").Inc()
		default:
			// Aborted (empalme), PermissionDenied, NotFound...: no se le puede inscribir.
			if err := p.descartar(ctx, *sig, msg); err != nil {
				return err
			}
		}
	}
}

// descartar cierra el lugar como "descartada". Devuelve error si la base falla,
// para que el consumidor reintente en vez de repetir el mismo candidato sin fin.
func (p *Promotor) descartar(ctx context.Context, e Espera, motivo string) error {
	ok, err := p.Store.Resolver(ctx, e.ID, "descartada", motivo)
	if err != nil {
		return err
	}
	if ok {
		log.Printf("espera: alumno %d descartado de %s: %s", e.AlumnoID, e.MateriaClave, motivo)
		resueltas.WithLabelValues("descartada").Inc()
		p.publicar(mensajeria.TipoEsperaDescartada, e, motivo)
	}
	return nil
}

func (p *Promotor) publicar(tipo string, e Espera, motivo string) {
	p.Productor.Publicar(mensajeria.TopicEspera, strconv.Itoa(e.MateriaID), tipo, mensajeria.EsperaEvento{
		EsperaID: e.ID, AlumnoID: e.AlumnoID, AlumnoNombre: e.AlumnoNombre, AlumnoCorreo: e.AlumnoCorreo,
		MateriaID: e.MateriaID, MateriaClave: e.MateriaClave, MateriaNombre: e.MateriaNombre,
		Posicion: e.Posicion, Motivo: motivo,
	})
}
