package controllers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"alumnos/models"
)

type AlumnoController struct {
	AlumnoModel *models.AlumnoModel
}

func (c *AlumnoController) AlumnosHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		list, err := c.AlumnoModel.GetAll()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if list == nil {
			list = []models.Alumno{}
		}
		json.NewEncoder(w).Encode(list)

	case http.MethodPost:
		var body models.Alumno
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		if body.Nombre == "" || body.Correo == "" || body.Password == "" || body.CarreraID == 0 {
			http.Error(w, "nombre, correo, contrasena y carrera son obligatorios", http.StatusBadRequest)
			return
		}
		item, err := c.AlumnoModel.Create(body)
		if err != nil && (strings.Contains(err.Error(), "ya esta registrado") || strings.Contains(err.Error(), "@up.edu.mx")) {
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

func (c *AlumnoController) AlumnoByIDHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	resto := strings.TrimPrefix(r.URL.Path, "/alumnos/")

	if resto == "login" {
		c.login(w, r)
		return
	}
	if resto == "carreras" {
		c.carreras(w, r)
		return
	}

	id, err := strconv.Atoi(resto)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	item, err := c.AlumnoModel.GetByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(item)
}

func (c *AlumnoController) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Correo   string `json:"correo"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	item, err := c.AlumnoModel.Login(body.Correo, body.Password)
	if errors.Is(err, models.ErrLogin) {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(item)
}

func (c *AlumnoController) carreras(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	list, err := c.AlumnoModel.GetCarreras()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if list == nil {
		list = []models.Carrera{}
	}
	json.NewEncoder(w).Encode(list)
}
