package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
)

// Cada worker al que le puedo mandar peticiones
type Worker struct {
	Nombre string
	Envio  *httputil.ReverseProxy
}

func leerVariable(nombre string, valorPorDefecto string) string {
	valor := os.Getenv(nombre)
	if valor == "" {
		return valorPorDefecto
	}
	return valor
}

func prepararWorker(nombre string, direccion string) Worker {
	direccionDelWorker, err := url.Parse(direccion)
	if err != nil {
		log.Fatal("La direccion del worker esta mal: ", err)
	}

	return Worker{
		Nombre: nombre,
		Envio:  httputil.NewSingleHostReverseProxy(direccionDelWorker),
	}
}

func main() {
	// Las direcciones me las pasa el docker-compose, si no las encuentra usa las de mi compu
	workerDeVistas := prepararWorker("vistas", leerVariable("WORKER_VISTAS", "http://localhost:8081"))
	workerDeTareas := prepararWorker("tareas", leerVariable("WORKER_TAREAS", "http://localhost:8082"))

	puerto := leerVariable("PORT", "8080")

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Aqui decido a que worker le toca la peticion segun la ruta que pidio el cliente
		workerQueLeToca := workerDeVistas
		if strings.HasPrefix(r.URL.Path, "/tareas") {
			workerQueLeToca = workerDeTareas
		}

		log.Printf("%s %s -> se lo mando al worker de %s", r.Method, r.URL.Path, workerQueLeToca.Nombre)

		workerQueLeToca.Envio.ServeHTTP(w, r)
	})

	log.Println("Middleware escuchando en el puerto " + puerto)
	log.Fatal(http.ListenAndServe(":"+puerto, nil))
}
