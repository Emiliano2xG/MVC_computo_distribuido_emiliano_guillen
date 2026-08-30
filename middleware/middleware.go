package middleware

import (
	"log"
	"net/http"
	"time"
)

// Sirve para ir viendo en la terminal que peticiones van llegando y cuanto tardan
func RegistrarPeticion(siguiente http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		momentoInicio := time.Now()

		siguiente(w, r)

		tiempoQueTardo := time.Since(momentoInicio)
		log.Printf("%s %s - tardo %v", r.Method, r.URL.Path, tiempoQueTardo)
	}
}

// Si algo truena adentro del handler, aqui lo agarramos para que no se caiga todo el servidor
func EvitarErrores(siguiente http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			problema := recover()
			if problema != nil {
				log.Println("Se cayo la peticion:", problema)
				http.Error(w, "Algo salio mal en el servidor", http.StatusInternalServerError)
			}
		}()

		siguiente(w, r)
	}
}
