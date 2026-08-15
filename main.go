package main

import (
	"fmt"
	"log"
	"net/http"
	"monolito_tarea_1/controlador"
	"monolito_tarea_1/modelo"
)

func main() {
	conexion, err := modelo.ConectarBaseDeDatos()
	if err != nil {
		log.Fatal(err)
	}
	defer conexion.Close()

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		controlador.MostrarInicio(w, r, conexion)
	})
	http.HandleFunc("/tareas", func(w http.ResponseWriter, r *http.Request) {
		controlador.AgregarTarea(w, r, conexion)
	})
	http.HandleFunc("/tareas/completar", func(w http.ResponseWriter, r *http.Request) {
		controlador.MarcarTarea(w, r, conexion)
	})

	fmt.Println("Servidor en http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
