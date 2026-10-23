package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"horarios/mensajeria"
)

// Registro es un evento ya guardado en la bitacora de auditoria.
type Registro struct {
	EventoID     string          `json:"evento_id"`
	Tipo         string          `json:"tipo"`
	Topico       string          `json:"topico"`
	Origen       string          `json:"origen"`
	Version      int             `json:"version"`
	OcurrioEn    time.Time       `json:"ocurrio_en"`
	RecibidoEn   time.Time       `json:"recibido_en"`
	ProcesadoPor string          `json:"procesado_por"`
	Datos        json.RawMessage `json:"datos"`
}

// Resumen son los totales de la bitacora.
type Resumen struct {
	Total        int            `json:"total"`
	PorTipo      map[string]int `json:"por_tipo"`
	PorTopico    map[string]int `json:"por_topico"`
	PorReplica   map[string]int `json:"por_replica_que_lo_proceso"`
	PrimerEvento *time.Time     `json:"primer_evento,omitempty"`
	UltimoEvento *time.Time     `json:"ultimo_evento,omitempty"`
}

// Store es la bitacora en Postgres. El id del evento es la llave primaria:
// si Kafka entrega el mismo evento dos veces (entrega "al menos una vez"), el
// segundo INSERT no hace nada. Asi el efecto es "exactamente una vez" aunque
// la entrega no lo sea (consumidor idempotente).
type Store struct {
	DB      *sql.DB
	Replica string // INSTANCE_ID de quien guarda
}

// Migrar crea la tabla si no existe. Con 2 replicas arrancando a la vez, el
// advisory lock evita que las dos intenten crearla al mismo tiempo.
func (s *Store) Migrar(ctx context.Context) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(947001)`); err != nil {
		return err
	}
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS auditoria_eventos (
			evento_id     VARCHAR(40)  PRIMARY KEY,
			tipo          VARCHAR(60)  NOT NULL,
			topico        VARCHAR(80)  NOT NULL,
			origen        VARCHAR(80)  NOT NULL,
			version       INTEGER      NOT NULL,
			ocurrio_en    TIMESTAMPTZ  NOT NULL,
			recibido_en   TIMESTAMPTZ  NOT NULL DEFAULT now(),
			procesado_por VARCHAR(80)  NOT NULL,
			datos         JSONB        NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_auditoria_tipo  ON auditoria_eventos (tipo)`,
		`CREATE INDEX IF NOT EXISTS idx_auditoria_fecha ON auditoria_eventos (ocurrio_en DESC)`,
	}
	for _, q := range stmts {
		if _, err := tx.ExecContext(ctx, q); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Guardar inserta el evento. Devuelve false si ya estaba (entrega repetida).
func (s *Store) Guardar(ctx context.Context, topico string, ev mensajeria.Evento) (bool, error) {
	datos := []byte(ev.Datos)
	if !json.Valid(datos) {
		datos = []byte("{}")
	}
	res, err := s.DB.ExecContext(ctx, `
		INSERT INTO auditoria_eventos (evento_id, tipo, topico, origen, version, ocurrio_en, procesado_por, datos)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (evento_id) DO NOTHING`,
		ev.ID, ev.Tipo, topico, ev.Origen, ev.Version, ev.Fecha, s.Replica, datos)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

// Listar devuelve los eventos mas recientes, con filtros opcionales.
func (s *Store) Listar(ctx context.Context, tipo, topico string, limite int) ([]Registro, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT evento_id, tipo, topico, origen, version, ocurrio_en, recibido_en, procesado_por, datos
		FROM auditoria_eventos
		WHERE ($1 = '' OR tipo = $1) AND ($2 = '' OR topico = $2)
		ORDER BY ocurrio_en DESC, evento_id
		LIMIT $3`, tipo, topico, limite)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Registro{}
	for rows.Next() {
		var r Registro
		var datos []byte
		if err := rows.Scan(&r.EventoID, &r.Tipo, &r.Topico, &r.Origen, &r.Version, &r.OcurrioEn, &r.RecibidoEn, &r.ProcesadoPor, &datos); err != nil {
			return nil, err
		}
		r.Datos = datos
		out = append(out, r)
	}
	return out, rows.Err()
}

// Resumen cuenta los eventos por tipo, topico y replica que los proceso.
func (s *Store) Resumen(ctx context.Context) (Resumen, error) {
	r := Resumen{PorTipo: map[string]int{}, PorTopico: map[string]int{}, PorReplica: map[string]int{}}

	var primero, ultimo sql.NullTime
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*), MIN(ocurrio_en), MAX(ocurrio_en) FROM auditoria_eventos`).
		Scan(&r.Total, &primero, &ultimo)
	if err != nil {
		return r, err
	}
	if primero.Valid {
		r.PrimerEvento = &primero.Time
		r.UltimoEvento = &ultimo.Time
	}

	for col, dst := range map[string]map[string]int{"tipo": r.PorTipo, "topico": r.PorTopico, "procesado_por": r.PorReplica} {
		rows, err := s.DB.QueryContext(ctx, `SELECT `+col+`, COUNT(*) FROM auditoria_eventos GROUP BY 1`)
		if err != nil {
			return r, err
		}
		for rows.Next() {
			var k string
			var n int
			if err := rows.Scan(&k, &n); err != nil {
				rows.Close()
				return r, err
			}
			dst[k] = n
		}
		rows.Close()
	}
	return r, nil
}
