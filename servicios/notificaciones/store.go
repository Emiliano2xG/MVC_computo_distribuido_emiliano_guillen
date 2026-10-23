package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"horarios/mensajeria"
)

const coordinacion = "coordinacion@up.edu.mx"

// Notificacion es lo que "se enviaria" a alguien por un evento. Aqui el
// envio es simulado: se escribe en el log y se guarda en memoria.
type Notificacion struct {
	EventoID     string    `json:"evento_id"`
	Tipo         string    `json:"tipo"`
	Topico       string    `json:"topico"`
	Destinatario string    `json:"destinatario"`
	Mensaje      string    `json:"mensaje"`
	Origen       string    `json:"origen"`
	OcurrioEn    time.Time `json:"ocurrio_en"`
	RecibidaEn   time.Time `json:"recibida_en"`
}

// Store guarda las ultimas N notificaciones (anillo en memoria) y recuerda
// los ids de evento ya vistos para ignorar entregas repetidas (Kafka entrega
// al menos una vez).
type Store struct {
	mu     sync.Mutex
	max    int
	lista  []Notificacion // la mas reciente al final
	vistos map[string]struct{}
	orden  []string
	total  int
}

func NewStore(max int) *Store {
	return &Store{max: max, vistos: map[string]struct{}{}}
}

// Ultimas devuelve hasta n notificaciones, de la mas reciente a la mas vieja.
func (s *Store) Ultimas(n int) ([]Notificacion, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n <= 0 || n > len(s.lista) {
		n = len(s.lista)
	}
	out := make([]Notificacion, 0, n)
	for i := len(s.lista) - 1; i >= len(s.lista)-n; i-- {
		out = append(out, s.lista[i])
	}
	return out, s.total
}

func (s *Store) agregar(n Notificacion) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ya := s.vistos[n.EventoID]; ya {
		return false
	}
	s.vistos[n.EventoID] = struct{}{}
	s.orden = append(s.orden, n.EventoID)
	if len(s.orden) > s.max*10 {
		delete(s.vistos, s.orden[0])
		s.orden = s.orden[1:]
	}
	s.lista = append(s.lista, n)
	if len(s.lista) > s.max {
		s.lista = s.lista[1:]
	}
	s.total++
	return true
}

// Manejar es el Handler del consumidor: convierte cada evento en una notificacion.
func (s *Store) Manejar(ctx context.Context, topic string, ev mensajeria.Evento) error {
	n := Notificacion{
		EventoID:   ev.ID,
		Tipo:       ev.Tipo,
		Topico:     topic,
		Origen:     ev.Origen,
		OcurrioEn:  ev.Fecha,
		RecibidaEn: time.Now().UTC(),
	}

	switch ev.Tipo {
	case mensajeria.TipoAlumnoCreado:
		var d mensajeria.AlumnoCreado
		if err := ev.Decodificar(&d); err != nil {
			return nil
		}
		n.Destinatario = d.Correo
		n.Mensaje = fmt.Sprintf("Bienvenido %s: tu cuenta (matricula %s) en %s ya esta lista.", d.Nombre, d.Matricula, d.Carrera)

	case mensajeria.TipoInscripcionCreada:
		var d mensajeria.InscripcionEvento
		if err := ev.Decodificar(&d); err != nil {
			return nil
		}
		n.Destinatario = destinoAlumno(d)
		n.Mensaje = fmt.Sprintf("Te inscribiste a %s - %s.", d.MateriaClave, d.MateriaNombre)

	case mensajeria.TipoInscripcionCancelada:
		var d mensajeria.InscripcionEvento
		if err := ev.Decodificar(&d); err != nil {
			return nil
		}
		n.Destinatario = destinoAlumno(d)
		n.Mensaje = fmt.Sprintf("Diste de baja %s - %s.", d.MateriaClave, d.MateriaNombre)

	case mensajeria.TipoMateriaCupoAgotado:
		var d mensajeria.MateriaCupo
		if err := ev.Decodificar(&d); err != nil {
			return nil
		}
		n.Destinatario = coordinacion
		n.Mensaje = fmt.Sprintf("Cupo agotado en %s - %s (%d/%d).", d.Clave, d.Nombre, d.Inscritos, d.Cupo)

	case mensajeria.TipoMateriaCupoLiberado:
		var d mensajeria.MateriaCupo
		if err := ev.Decodificar(&d); err != nil {
			return nil
		}
		n.Destinatario = coordinacion
		n.Mensaje = fmt.Sprintf("Se libero un lugar en %s - %s (%d/%d).", d.Clave, d.Nombre, d.Inscritos, d.Cupo)

	default:
		log.Printf("notificaciones: evento desconocido %q (id=%s), se ignora", ev.Tipo, ev.ID)
		return nil
	}

	if !s.agregar(n) {
		log.Printf("notificaciones: evento repetido %s (id=%s), se ignora", ev.Tipo, ev.ID)
		return nil
	}
	log.Printf("NOTIFICACION [%s] para %s: %s", n.Tipo, n.Destinatario, n.Mensaje)
	return nil
}

func destinoAlumno(d mensajeria.InscripcionEvento) string {
	if d.AlumnoCorreo != "" {
		return d.AlumnoCorreo
	}
	return fmt.Sprintf("alumno-%d", d.AlumnoID)
}
