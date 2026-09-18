package models

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/lib/pq"
)

var ErrLogin = errors.New("correo o contrasena incorrectos")

type Carrera struct {
	ID     int    `json:"id"`
	Clave  string `json:"clave"`
	Nombre string `json:"nombre"`
}

type Alumno struct {
	ID        int    `json:"id"`
	Nombre    string `json:"nombre"`
	Matricula string `json:"matricula"`
	Correo    string `json:"correo"`
	Password  string `json:"password,omitempty"`
	CarreraID int    `json:"carrera_id"`
	Carrera   string `json:"carrera,omitempty"`
}

type AlumnoModel struct {
	DB *sql.DB
}

func (m *AlumnoModel) GetCarreras() ([]Carrera, error) {
	rows, err := m.DB.Query(`SELECT id, clave, nombre FROM carreras ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Carrera
	for rows.Next() {
		var item Carrera
		if err = rows.Scan(&item.ID, &item.Clave, &item.Nombre); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, nil
}

func (m *AlumnoModel) scanAlumno(row interface{ Scan(dest ...any) error }) (*Alumno, error) {
	item := &Alumno{}
	err := row.Scan(&item.ID, &item.Nombre, &item.Matricula, &item.Correo, &item.CarreraID, &item.Carrera)
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (m *AlumnoModel) GetAll() ([]Alumno, error) {
	query := `
		SELECT a.id, a.nombre, a.matricula, a.correo, a.carrera_id, c.nombre
		FROM alumnos a
		JOIN carreras c ON c.id = a.carrera_id
		ORDER BY a.id`
	rows, err := m.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Alumno
	for rows.Next() {
		item, err := m.scanAlumno(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *item)
	}
	return list, nil
}

func (m *AlumnoModel) GetByID(id int) (*Alumno, error) {
	query := `
		SELECT a.id, a.nombre, a.matricula, a.correo, a.carrera_id, c.nombre
		FROM alumnos a
		JOIN carreras c ON c.id = a.carrera_id
		WHERE a.id = $1`
	return m.scanAlumno(m.DB.QueryRow(query, id))
}

func (m *AlumnoModel) Login(correo string, password string) (*Alumno, error) {
	query := `
		SELECT a.id, a.nombre, a.matricula, a.correo, a.carrera_id, c.nombre
		FROM alumnos a
		JOIN carreras c ON c.id = a.carrera_id
		WHERE lower(a.correo) = lower($1) AND a.password = $2`
	item, err := m.scanAlumno(m.DB.QueryRow(query, strings.TrimSpace(correo), password))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrLogin
	}
	return item, err
}

func (m *AlumnoModel) Create(item Alumno) (*Alumno, error) {
	item.Correo = strings.ToLower(strings.TrimSpace(item.Correo))
	if !strings.HasSuffix(item.Correo, "@up.edu.mx") {
		return nil, errors.New("el correo debe ser @up.edu.mx")
	}
	if item.Matricula == "" {
		item.Matricula = strings.Split(item.Correo, "@")[0]
	}
	query := `
		INSERT INTO alumnos (nombre, matricula, correo, password, carrera_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`
	err := m.DB.QueryRow(query, item.Nombre, item.Matricula, item.Correo, item.Password, item.CarreraID).Scan(&item.ID)
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" {
		return nil, errors.New("esa matricula o correo ya esta registrado")
	}
	if err != nil {
		return nil, err
	}
	item.Password = ""
	creado, err := m.GetByID(item.ID)
	if err != nil {
		return &item, nil
	}
	return creado, nil
}
