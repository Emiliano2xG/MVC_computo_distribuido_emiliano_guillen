package models

import (
	"database/sql"
)

type Materia struct {
	ID        int    `json:"id"`
	Clave     string `json:"clave"`
	Nombre    string `json:"nombre"`
	Profesor  string `json:"profesor"`
	Cupo      int    `json:"cupo"`
	Inscritos int    `json:"inscritos"`
	Horario   string `json:"horario"`
	Dias      string `json:"dias"`
	Semestre  int    `json:"semestre"`
}

type MateriaModel struct {
	DB *sql.DB
}

func (m *MateriaModel) leerFilas(rows *sql.Rows) ([]Materia, error) {
	defer rows.Close()
	var list []Materia
	for rows.Next() {
		var item Materia
		err := rows.Scan(&item.ID, &item.Clave, &item.Nombre, &item.Profesor, &item.Cupo, &item.Inscritos, &item.Horario, &item.Dias, &item.Semestre)
		if err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, nil
}

func (m *MateriaModel) GetAll() ([]Materia, error) {
	query := `
		SELECT m.id, m.clave, m.nombre, m.profesor, m.cupo,
		       (SELECT COUNT(*) FROM inscripciones i WHERE i.materia_id = m.id AND i.estado = 'inscrita') AS inscritos,
		       m.horario, m.dias, m.semestre
		FROM materias m
		ORDER BY m.semestre, m.id`
	rows, err := m.DB.Query(query)
	if err != nil {
		return nil, err
	}
	return m.leerFilas(rows)
}

func (m *MateriaModel) GetByCarreraID(carreraID int) ([]Materia, error) {
	query := `
		SELECT m.id, m.clave, m.nombre, m.profesor, m.cupo,
		       (SELECT COUNT(*) FROM inscripciones i WHERE i.materia_id = m.id AND i.estado = 'inscrita') AS inscritos,
		       m.horario, m.dias, m.semestre
		FROM materias m
		JOIN carrera_materias cm ON cm.materia_id = m.id
		WHERE cm.carrera_id = $1
		ORDER BY m.semestre, m.id`
	rows, err := m.DB.Query(query, carreraID)
	if err != nil {
		return nil, err
	}
	return m.leerFilas(rows)
}

func (m *MateriaModel) GetByID(id int) (*Materia, error) {
	item := &Materia{}
	query := `SELECT id, clave, nombre, profesor, cupo, horario, dias, semestre FROM materias WHERE id = $1`
	err := m.DB.QueryRow(query, id).Scan(&item.ID, &item.Clave, &item.Nombre, &item.Profesor, &item.Cupo, &item.Horario, &item.Dias, &item.Semestre)
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (m *MateriaModel) Create(item Materia) (*Materia, error) {
	query := `INSERT INTO materias (clave, nombre, profesor, cupo, horario, dias) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`
	err := m.DB.QueryRow(query, item.Clave, item.Nombre, item.Profesor, item.Cupo, item.Horario, item.Dias).Scan(&item.ID)
	if err != nil {
		return nil, err
	}
	return &item, nil
}
