// Package cupos es el consumidor de Kafka del servicio de materias.
//
// Escucha inscripciones.eventos (inscripcion.creada / inscripcion.cancelada),
// revisa cuantos lugares le quedan a la materia y, cuando cambia de estado,
// publica en su propio topico (materias.eventos):
//
//	materia.cupo_agotado   la materia acaba de llenarse
//	materia.cupo_liberado  se libero un lugar en una materia que estaba llena
package cupos

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"strconv"
	"sync"

	"materias/models"

	"horarios/mensajeria"
)

// GrupoConsumidor: todas las replicas de materias comparten grupo, asi Kafka
// reparte las particiones entre ellas y cada materia (llave = materia_id) la
// procesa siempre la misma replica, en orden.
const GrupoConsumidor = "materias-cupos"

type Monitor struct {
	Model     *models.MateriaModel
	Productor *mensajeria.Productor

	mu    sync.Mutex
	llena map[int]bool // ultimo estado conocido de cada materia
}

func New(model *models.MateriaModel, p *mensajeria.Productor) *Monitor {
	return &Monitor{Model: model, Productor: p, llena: map[int]bool{}}
}

// Iniciar carga el estado actual desde la base y empieza a consumir.
// Bloquea hasta que ctx se cancele (llamarlo en una goroutine).
func (m *Monitor) Iniciar(ctx context.Context) {
	if todas, err := m.Model.GetAll(); err == nil {
		m.mu.Lock()
		for _, mat := range todas {
			m.llena[mat.ID] = mat.Inscritos >= mat.Cupo
		}
		m.mu.Unlock()
	} else {
		log.Printf("cupos: no se pudo cargar el estado inicial: %v", err)
	}
	mensajeria.Consumir(ctx, GrupoConsumidor, []string{mensajeria.TopicInscripciones}, m.manejar)
}

func (m *Monitor) manejar(ctx context.Context, topic string, ev mensajeria.Evento) error {
	if ev.Tipo != mensajeria.TipoInscripcionCreada && ev.Tipo != mensajeria.TipoInscripcionCancelada {
		return nil // no es de mi interes
	}
	var d mensajeria.InscripcionEvento
	if err := ev.Decodificar(&d); err != nil {
		return nil // evento mal formado: reintentar no sirve
	}

	mat, err := m.Model.Ocupacion(d.MateriaID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err // se reintenta
	}
	lleno := mat.Inscritos >= mat.Cupo

	m.mu.Lock()
	antes := m.llena[mat.ID]
	m.llena[mat.ID] = lleno
	m.mu.Unlock()

	datos := mensajeria.MateriaCupo{
		MateriaID: mat.ID, Clave: mat.Clave, Nombre: mat.Nombre, Cupo: mat.Cupo, Inscritos: mat.Inscritos,
	}
	key := strconv.Itoa(mat.ID)
	switch {
	case lleno && !antes:
		log.Printf("cupos: %s (%s) se lleno: %d/%d", mat.Clave, mat.Nombre, mat.Inscritos, mat.Cupo)
		m.Productor.Publicar(mensajeria.TopicMaterias, key, mensajeria.TipoMateriaCupoAgotado, datos)
	case !lleno && antes:
		log.Printf("cupos: %s (%s) libero un lugar: %d/%d", mat.Clave, mat.Nombre, mat.Inscritos, mat.Cupo)
		m.Productor.Publicar(mensajeria.TopicMaterias, key, mensajeria.TipoMateriaCupoLiberado, datos)
	}
	return nil
}
