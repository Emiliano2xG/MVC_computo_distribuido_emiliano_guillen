package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"materias/models"
)

type MateriaController struct {
	MateriaModel *models.MateriaModel
}

func (c *MateriaController) MateriasHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		var list []models.Materia
		var err error
		if carrera := r.URL.Query().Get("carrera_id"); carrera != "" {
			id, convErr := strconv.Atoi(carrera)
			if convErr != nil {
				http.Error(w, "carrera_id invalido", http.StatusBadRequest)
				return
			}
			list, err = c.MateriaModel.GetByCarreraID(id)
		} else {
			list, err = c.MateriaModel.GetAll()
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if list == nil {
			list = []models.Materia{}
		}
		json.NewEncoder(w).Encode(list)

	case http.MethodPost:
		var body models.Materia
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		item, err := c.MateriaModel.Create(body)
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

func (c *MateriaController) MateriaByIDHandler(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/materias/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	item, err := c.MateriaModel.GetByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(item)
}
