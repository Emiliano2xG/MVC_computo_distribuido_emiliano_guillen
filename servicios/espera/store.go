package main

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/lib/pq"
)

var ErrYaEnFila = errors.New("ya estas en la lista de espera de esa materia")

// Espera es un lugar en la fila de una materia.
type Espera struct {
	ID            int       `json:"id"`
	AlumnoID      int       `json:"alumno_id"`
	MateriaID     int       `json:"materia_id"`
	MateriaClave  string    `json:"materia_clave"`
	MateriaNombre string    `json:"materia_nombre"`
	Estado        string    `json:"estado"`
	Posicion      int       `json:"posicion"`
	EnFila        int       `json:"en_fila"` // cuantos esperan esa materia en total
	CreadaEn      time.Time `json:"creada_en"`
	Motivo        string    `json:"motivo,omitempty"`

	// Se guardan para poder armar eventos sin volver a preguntar a alumnos.
	AlumnoNombre string `json:"-"`
	AlumnoCorreo string `json:"-"`
}

// Store es la fila de espera en Postgres.
//
// Estados: esperando -> inscrito (se le dio el lugar) | descartada (le toco
// pero ya no se pudo, ver motivo) | cancelada (el alumno salio).
//
// No hay llaves foraneas hacia alumnos/materias: son tablas de otros
// servicios y esas relaciones se validan por gRPC.
type Store struct{ DB *sql.DB }

// Migrar crea la tabla. El advisory lock evita choques si dos instancias
// arrancan a la vez.
func (s *Store) Migrar(ctx context.Context) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(947002)`); err != nil {
		return err
	}
	for _, q := range []string{
		`CREATE TABLE IF NOT EXISTS lista_espera (
			id             SERIAL PRIMARY KEY,
			alumno_id      INTEGER      NOT NULL,
			materia_id     INTEGER      NOT NULL,
			materia_clave  VARCHAR(20)  NOT NULL,
			materia_nombre VARCHAR(255) NOT NULL,
			alumno_nombre  VARCHAR(255) NOT NULL DEFAULT '',
			alumno_correo  VARCHAR(120) NOT NULL DEFAULT '',
			estado         VARCHAR(20)  NOT NULL DEFAULT 'esperando',
			motivo         VARCHAR(255) NOT NULL DEFAULT '',
			creada_en      TIMESTAMPTZ  NOT NULL DEFAULT now(),
			resuelta_en    TIMESTAMPTZ
		)`,
		// Un alumno solo puede estar una vez "esperando" en la misma materia.
		`CREATE UNIQUE INDEX IF NOT EXISTS ux_espera_activa ON lista_espera (alumno_id, materia_id) WHERE estado = 'esperando'`,
		`CREATE INDEX IF NOT EXISTS idx_espera_fila ON lista_espera (materia_id, id) WHERE estado = 'esperando'`,
	} {
		if _, err := tx.ExecContext(ctx, q); err != nil {
			return err
		}
	}
	return tx.Commit()
}

const columnas = `e.id, e.alumno_id, e.materia_id, e.materia_clave, e.materia_nombre, e.estado, e.motivo, e.creada_en,
	e.alumno_nombre, e.alumno_correo,
	(SELECT COUNT(*) FROM lista_espera x WHERE x.materia_id = e.materia_id AND x.estado = 'esperando' AND x.id <= e.id),
	(SELECT COUNT(*) FROM lista_espera x WHERE x.materia_id = e.materia_id AND x.estado = 'esperando')`

func leer(sc interface{ Scan(...any) error }) (Espera, error) {
	var e Espera
	err := sc.Scan(&e.ID, &e.AlumnoID, &e.MateriaID, &e.MateriaClave, &e.MateriaNombre, &e.Estado, &e.Motivo, &e.CreadaEn,
		&e.AlumnoNombre, &e.AlumnoCorreo, &e.Posicion, &e.EnFila)
	return e, err
}

// Crear mete al alumno al final de la fila. ErrYaEnFila si ya estaba esperando.
func (s *Store) Crear(ctx context.Context, e Espera) (Espera, error) {
	var id int
	err := s.DB.QueryRowContext(ctx, `
		INSERT INTO lista_espera (alumno_id, materia_id, materia_clave, materia_nombre, alumno_nombre, alumno_correo)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		e.AlumnoID, e.MateriaID, e.MateriaClave, e.MateriaNombre, e.AlumnoNombre, e.AlumnoCorreo).Scan(&id)
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" {
		return Espera{}, ErrYaEnFila
	}
	if err != nil {
		return Espera{}, err
	}
	return s.Obtener(ctx, id)
}

func (s *Store) Obtener(ctx context.Context, id int) (Espera, error) {
	return leer(s.DB.QueryRowContext(ctx, `SELECT `+columnas+` FROM lista_espera e WHERE e.id = $1`, id))
}

// ListarPorAlumno devuelve donde esta formado el alumno (solo lo que sigue esperando).
func (s *Store) ListarPorAlumno(ctx context.Context, alumnoID int) ([]Espera, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT `+columnas+` FROM lista_espera e WHERE e.alumno_id = $1 AND e.estado = 'esperando' ORDER BY e.id`, alumnoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Espera{}
	for rows.Next() {
		e, err := leer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Conteo devuelve cuantos esperan cada materia (materia_id -> total).
func (s *Store) Conteo(ctx context.Context) (map[int]int, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT materia_id, COUNT(*) FROM lista_espera WHERE estado = 'esperando' GROUP BY 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int]int{}
	for rows.Next() {
		var m, n int
		if err := rows.Scan(&m, &n); err != nil {
			return nil, err
		}
		out[m] = n
	}
	return out, rows.Err()
}

// Siguiente es el primero de la fila de una materia (nil si nadie espera).
func (s *Store) Siguiente(ctx context.Context, materiaID int) (*Espera, error) {
	e, err := leer(s.DB.QueryRowContext(ctx,
		`SELECT `+columnas+` FROM lista_espera e WHERE e.materia_id = $1 AND e.estado = 'esperando' ORDER BY e.id LIMIT 1`, materiaID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// Resolver cierra un lugar de la fila con el estado dado. Solo cierra lo que
// sigue "esperando": asi un mensaje repetido de Kafka no lo cierra dos veces.
func (s *Store) Resolver(ctx context.Context, id int, estado, motivo string) (bool, error) {
	res, err := s.DB.ExecContext(ctx,
		`UPDATE lista_espera SET estado = $2, motivo = $3, resuelta_en = now() WHERE id = $1 AND estado = 'esperando'`,
		id, estado, motivo)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

// Cancelar es salir de la fila por decision del alumno. sql.ErrNoRows si no
// existe o ya no estaba esperando.
func (s *Store) Cancelar(ctx context.Context, id int) (Espera, error) {
	e, err := s.Obtener(ctx, id)
	if err != nil {
		return Espera{}, err
	}
	ok, err := s.Resolver(ctx, id, "cancelada", "")
	if err != nil {
		return Espera{}, err
	}
	if !ok {
		return Espera{}, sql.ErrNoRows
	}
	return e, nil
}
