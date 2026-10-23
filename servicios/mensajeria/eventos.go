// Package mensajeria es la capa de mensajeria asincrona (Apache Kafka) que
// comparten los servicios. Define los topicos, el formato de los eventos y
// un productor / consumidor sencillos sobre segmentio/kafka-go.
package mensajeria

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"time"
)

// Un topico por servicio: cada servicio publica en el suyo los hechos de su
// dominio y los demas se suscriben a los que les interesan.
const (
	TopicAlumnos       = "alumnos.eventos"
	TopicMaterias      = "materias.eventos"
	TopicInscripciones = "inscripciones.eventos"
	TopicEspera        = "espera.eventos"
)

// Topics lista todos los topicos (lo usa el init del compose y la documentacion).
var Topics = []string{TopicAlumnos, TopicMaterias, TopicInscripciones, TopicEspera}

// Tipos de evento (campo "tipo" del sobre).
const (
	TipoAlumnoCreado         = "alumno.creado"
	TipoInscripcionCreada    = "inscripcion.creada"
	TipoInscripcionCancelada = "inscripcion.cancelada"
	TipoMateriaCupoAgotado   = "materia.cupo_agotado"
	TipoMateriaCupoLiberado  = "materia.cupo_liberado"

	// Lista de espera (topico espera.eventos).
	TipoEsperaRegistrada = "espera.registrada" // el alumno se anoto en la fila
	TipoEsperaPromovida  = "espera.promovida"  // se libero un lugar y se le inscribio solo
	TipoEsperaDescartada = "espera.descartada" // le tocaba, pero ya no se pudo inscribir (ver motivo)
	TipoEsperaCancelada  = "espera.cancelada"  // el alumno salio de la fila
)

// Evento es el sobre comun de todos los mensajes (JSON en el value del
// mensaje de Kafka).
type Evento struct {
	ID      string          `json:"id"`      // unico; los consumidores lo usan para ignorar repetidos
	Tipo    string          `json:"tipo"`    // ver Tipo*
	Version int             `json:"version"` // version del esquema de "datos"
	Fecha   time.Time       `json:"fecha"`   // cuando ocurrio
	Origen  string          `json:"origen"`  // replica que lo publico (INSTANCE_ID)
	Datos   json.RawMessage `json:"datos"`
}

// Decodificar lee "datos" en la estructura que se indique.
func (e Evento) Decodificar(v any) error { return json.Unmarshal(e.Datos, v) }

// NuevoEvento arma el sobre con id y fecha.
func NuevoEvento(tipo, origen string, datos any) (Evento, error) {
	raw, err := json.Marshal(datos)
	if err != nil {
		return Evento{}, err
	}
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return Evento{}, err
	}
	return Evento{
		ID:      hex.EncodeToString(b),
		Tipo:    tipo,
		Version: 1,
		Fecha:   time.Now().UTC(),
		Origen:  origen,
		Datos:   raw,
	}, nil
}

// ---- Carga util de cada tipo de evento ----

// AlumnoCreado: topico alumnos.eventos, llave = alumno_id.
type AlumnoCreado struct {
	AlumnoID  int    `json:"alumno_id"`
	Nombre    string `json:"nombre"`
	Matricula string `json:"matricula"`
	Correo    string `json:"correo"`
	CarreraID int    `json:"carrera_id"`
	Carrera   string `json:"carrera"`
}

// InscripcionEvento: topico inscripciones.eventos, llave = materia_id (asi
// todos los eventos de una materia caen en la misma particion y se procesan
// en orden). Sirve para inscripcion.creada y inscripcion.cancelada.
type InscripcionEvento struct {
	InscripcionID int    `json:"inscripcion_id"`
	AlumnoID      int    `json:"alumno_id"`
	AlumnoNombre  string `json:"alumno_nombre,omitempty"`
	AlumnoCorreo  string `json:"alumno_correo,omitempty"`
	MateriaID     int    `json:"materia_id"`
	MateriaClave  string `json:"materia_clave,omitempty"`
	MateriaNombre string `json:"materia_nombre,omitempty"`
}

// MateriaCupo: topico materias.eventos, llave = materia_id. Sirve para
// materia.cupo_agotado y materia.cupo_liberado.
type MateriaCupo struct {
	MateriaID int    `json:"materia_id"`
	Clave     string `json:"clave"`
	Nombre    string `json:"nombre"`
	Cupo      int    `json:"cupo"`
	Inscritos int    `json:"inscritos"`
}

// EsperaEvento: topico espera.eventos, llave = materia_id. Sirve para los
// cuatro tipos de espera.*. Posicion solo viene en espera.registrada y Motivo
// en espera.descartada.
type EsperaEvento struct {
	EsperaID      int    `json:"espera_id"`
	AlumnoID      int    `json:"alumno_id"`
	AlumnoNombre  string `json:"alumno_nombre,omitempty"`
	AlumnoCorreo  string `json:"alumno_correo,omitempty"`
	MateriaID     int    `json:"materia_id"`
	MateriaClave  string `json:"materia_clave,omitempty"`
	MateriaNombre string `json:"materia_nombre,omitempty"`
	Posicion      int    `json:"posicion,omitempty"`
	Motivo        string `json:"motivo,omitempty"`
}
