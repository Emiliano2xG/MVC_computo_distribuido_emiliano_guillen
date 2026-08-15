package controlador

import (
	"database/sql"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"monolito_tarea_1/modelo"
)

func MostrarInicio(w http.ResponseWriter, r *http.Request, conexion *sql.DB) {
	todasLasTareas, err := modelo.TraerTareas(conexion)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	plantilla := template.Must(template.ParseFiles("vista/index.html"))
	plantilla.Execute(w, todasLasTareas)
}

func AgregarTarea(w http.ResponseWriter, r *http.Request, conexion *sql.DB) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	tituloNuevaTarea := strings.TrimSpace(r.FormValue("tarea"))
	if tituloNuevaTarea != "" {
		_ = modelo.GuardarTarea(conexion, tituloNuevaTarea)
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func MarcarTarea(w http.ResponseWriter, r *http.Request, conexion *sql.DB) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	idTarea, err := strconv.Atoi(r.FormValue("id"))
	if err == nil {
		_ = modelo.CambiarEstadoTarea(conexion, idTarea)
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
