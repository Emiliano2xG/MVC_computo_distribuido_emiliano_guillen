package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"monolito_tarea_1/controlador"
	"monolito_tarea_1/filtros"
	"monolito_tarea_1/modelo"
)

func main() {
	conexion, err := modelo.ConectarBaseDeDatos()
	if err != nil {
		log.Fatal(err)
	}
	defer conexion.Close()

	// El nombre me sirve para saber cual worker contesto cuando hago las pruebas
	nombreDelWorker := os.Getenv("NOMBRE_WORKER")
	if nombreDelWorker == "" {
		nombreDelWorker = "worker"
	}

	puerto := os.Getenv("PORT")
	if puerto == "" {
		puerto = "8080"
	}

	// Asi no tengo que repetir los filtros en cada ruta
	conFiltros := func(manejador http.HandlerFunc) http.HandlerFunc {
		return filtros.RegistrarPeticion(filtros.EvitarErrores(manejador))
	}

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	http.HandleFunc("/", conFiltros(func(w http.ResponseWriter, r *http.Request) {
		controlador.MostrarInicio(w, r, conexion)
	}))
	http.HandleFunc("/tareas", conFiltros(func(w http.ResponseWriter, r *http.Request) {
		controlador.AgregarTarea(w, r, conexion)
	}))
	http.HandleFunc("/tareas/completar", conFiltros(func(w http.ResponseWriter, r *http.Request) {
		controlador.MarcarTarea(w, r, conexion)
	}))

	fmt.Printf("Worker de %s escuchando en el puerto %s\n", nombreDelWorker, puerto)
	log.Fatal(http.ListenAndServe(":"+puerto, nil))
}
