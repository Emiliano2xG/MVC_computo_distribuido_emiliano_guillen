package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"inscripciones/controllers"
	"inscripciones/models"

	_ "github.com/lib/pq"
)

var instanceID = "unknown"

func withInstanceHeader(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Instance-Id", instanceID)
		next(w, r)
	}
}

func heartbeatHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("backend alive"))
}

func main() {
	if v := os.Getenv("INSTANCE_ID"); v != "" {
		instanceID = v
	}

	dbHost := os.Getenv("DB_HOST")
	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	dbPort := os.Getenv("DB_PORT")

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPass, dbName)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Failed to configure database:", err)
	}
	defer db.Close()

	var pingErr error
	for i := 0; i < 15; i++ {
		pingErr = db.Ping()
		if pingErr == nil {
			break
		}
		time.Sleep(time.Second)
	}
	if pingErr != nil {
		log.Fatal("Failed to ping database:", pingErr)
	}

	// Initialize MVC Components
	inscripcionModel := &models.InscripcionModel{DB: db}
	inscripcionController := &controllers.InscripcionController{InscripcionModel: inscripcionModel}

	mux := http.NewServeMux()
	mux.HandleFunc("/inscripciones", withInstanceHeader(inscripcionController.InscripcionesHandler))
	mux.HandleFunc("/inscripciones/", withInstanceHeader(inscripcionController.BajaHandler))
	mux.HandleFunc("/heartbeat", withInstanceHeader(heartbeatHandler))

	log.Printf("backend %s listening on :8080", instanceID)
	log.Fatal(http.ListenAndServe(":8080", mux))
}
