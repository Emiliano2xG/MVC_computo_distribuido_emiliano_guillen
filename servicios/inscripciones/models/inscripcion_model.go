package models

import (
	"database/sql"
	"errors"

	"github.com/lib/pq"
)

var ErrSinCupo = errors.New("ya no hay cupo en esa materia")
var ErrYaInscrito = errors.New("el alumno ya esta inscrito en esa materia")
var ErrYaCursada = errors.New("esa materia ya esta cursada")
var ErrOtraCarrera = errors.New("esa materia no es de tu carrera")

type Inscripcion struct {
	ID        int    `json:"id"`
	AlumnoID  int    `json:"alumno_id"`
	MateriaID int    `json:"materia_id"`
	Estado    string `json:"estado"`
	Materia   string `json:"materia,omitempty"`
	Clave     string `json:"clave,omitempty"`
	Profesor  string `json:"profesor,omitempty"`
	Horario   string `json:"horario,omitempty"`
	Dias      string `json:"dias,omitempty"`
}

type InscripcionModel struct {
	DB *sql.DB
}

func (m *InscripcionModel) GetByAlumnoID(alumnoID int) ([]Inscripcion, error) {
	query := `
		SELECT i.id, i.alumno_id, i.materia_id, i.estado, ma.nombre, ma.clave, ma.profesor, ma.horario, ma.dias
		FROM inscripciones i
		JOIN materias ma ON ma.id = i.materia_id
		WHERE i.alumno_id = $1
		ORDER BY i.id`
	rows, err := m.DB.Query(query, alumnoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Inscripcion
	for rows.Next() {
		var item Inscripcion
		err = rows.Scan(&item.ID, &item.AlumnoID, &item.MateriaID, &item.Estado, &item.Materia, &item.Clave, &item.Profesor, &item.Horario, &item.Dias)
		if err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, nil
}

func esLlaveDuplicada(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505"
}

func (m *InscripcionModel) Create(alumnoID int, materiaID int) (*Inscripcion, error) {
	tx, err := m.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var vaConCarrera int
	err = tx.QueryRow(`
		SELECT COUNT(*) FROM carrera_materias cm
		JOIN alumnos a ON a.carrera_id = cm.carrera_id
		WHERE a.id = $1 AND cm.materia_id = $2`, alumnoID, materiaID).Scan(&vaConCarrera)
	if err != nil {
		return nil, err
	}
	if vaConCarrera == 0 {
		return nil, ErrOtraCarrera
	}

	var estadoExistente string
	err = tx.QueryRow(`SELECT estado FROM inscripciones WHERE alumno_id = $1 AND materia_id = $2`, alumnoID, materiaID).Scan(&estadoExistente)
	if err == nil {
		if estadoExistente == "cursada" {
			return nil, ErrYaCursada
		}
		return nil, ErrYaInscrito
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	var cupo int
	err = tx.QueryRow(`SELECT cupo FROM materias WHERE id = $1 FOR UPDATE`, materiaID).Scan(&cupo)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("la materia no existe")
	}
	if err != nil {
		return nil, err
	}

	var ocupados int
	err = tx.QueryRow(`SELECT COUNT(*) FROM inscripciones WHERE materia_id = $1 AND estado = 'inscrita'`, materiaID).Scan(&ocupados)
	if err != nil {
		return nil, err
	}
	if ocupados >= cupo {
		return nil, ErrSinCupo
	}

	item := &Inscripcion{}
	err = tx.QueryRow(
		`INSERT INTO inscripciones (alumno_id, materia_id, estado) VALUES ($1, $2, 'inscrita') RETURNING id, alumno_id, materia_id, estado`,
		alumnoID, materiaID,
	).Scan(&item.ID, &item.AlumnoID, &item.MateriaID, &item.Estado)
	if esLlaveDuplicada(err) {
		return nil, ErrYaInscrito
	}
	if err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		if esLlaveDuplicada(err) {
			return nil, ErrYaInscrito
		}
		return nil, err
	}
	return item, nil
}

func (m *InscripcionModel) Delete(id int) error {
	var estado string
	err := m.DB.QueryRow(`SELECT estado FROM inscripciones WHERE id = $1`, id).Scan(&estado)
	if errors.Is(err, sql.ErrNoRows) {
		return sql.ErrNoRows
	}
	if err != nil {
		return err
	}
	if estado == "cursada" {
		return ErrYaCursada
	}

	result, err := m.DB.Exec(`DELETE FROM inscripciones WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
