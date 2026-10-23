// Servicio de notificaciones: consumidor asincrono de los tres topicos de
// Kafka. No tiene base de datos: cada evento se convierte en una notificacion
// (envio simulado en el log) y las ultimas se pueden ver por REST.
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"

	"horarios/discovery"
	"horarios/mensajeria"
)

const grupo = "notificaciones"

func main() {
	instanceID := os.Getenv("INSTANCE_ID")
	if instanceID == "" {
		instanceID = "notificaciones"
	}

	store := NewStore(200)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mensajeria.Consumir(ctx, grupo, mensajeria.Topics, store.Manejar)

	mux := http.NewServeMux()
	mux.HandleFunc("/notificaciones", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		limite, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		lista, total := store.Ultimas(limite)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Instance-Id", instanceID)
		w.Header().Set("X-Total-Recibidas", strconv.Itoa(total))
		json.NewEncoder(w).Encode(lista)
	})
	mux.HandleFunc("/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("backend alive"))
	})
	mux.HandleFunc("/health", discovery.HealthHandler(nil))

	log.Printf("backend %s: REST :8080 + consumidor Kafka (grupo %s)", instanceID, grupo)
	if err := discovery.Run(discovery.Config{Name: "notificaciones", Handler: mux}); err != nil {
		log.Fatal(err)
	}
}
