package controllers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"inscripciones/models"
)

type InscripcionController struct {
	InscripcionModel *models.InscripcionModel
}

func (c *InscripcionController) InscripcionesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		idStr := r.URL.Query().Get("alumno_id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, "Missing 'alumno_id' query parameter", http.StatusBadRequest)
			return
		}
		list, err := c.InscripcionModel.GetByAlumnoID(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if list == nil {
			list = []models.Inscripcion{}
		}
		json.NewEncoder(w).Encode(list)

	case http.MethodPost:
		var body struct {
			AlumnoID  int `json:"alumno_id"`
			MateriaID int `json:"materia_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		item, err := c.InscripcionModel.Create(body.AlumnoID, body.MateriaID)
		if errors.Is(err, models.ErrOtraCarrera) {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		if errors.Is(err, models.ErrSinCupo) || errors.Is(err, models.ErrYaInscrito) || errors.Is(err, models.ErrYaCursada) {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(item)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (c *InscripcionController) BajaHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/inscripciones/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	err = c.InscripcionModel.Delete(id)
	if errors.Is(err, models.ErrYaCursada) {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
