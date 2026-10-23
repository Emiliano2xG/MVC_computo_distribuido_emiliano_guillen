package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"inscripciones/clients"
	"inscripciones/controllers"
	"inscripciones/grpcserver"
	"inscripciones/models"
	"inscripciones/service"

	"horarios/discovery"
	"horarios/mensajeria"
	pb "horarios/pb/inscripciones"

	"google.golang.org/grpc"

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

	// Clientes gRPC hacia alumnos y materias (pasan por el load balancer :9000).
	cl, err := clients.New()
	if err != nil {
		log.Fatal("Failed to create gRPC clients:", err)
	}
	defer cl.Close()

	// Productor Kafka: publica inscripcion.creada / inscripcion.cancelada.
	productor := mensajeria.NuevoProductor()
	defer productor.Cerrar()
	inscripcionService := service.New(inscripcionModel, cl, productor)

	inscripcionController := &controllers.InscripcionController{
		InscripcionModel: inscripcionModel,
		Service:          inscripcionService,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/inscripciones", withInstanceHeader(inscripcionController.InscripcionesHandler))
	mux.HandleFunc("/inscripciones/", withInstanceHeader(inscripcionController.BajaHandler))
	mux.HandleFunc("/heartbeat", withInstanceHeader(heartbeatHandler))
	mux.HandleFunc("/health", withInstanceHeader(discovery.HealthHandler(db.Ping)))

	log.Printf("backend %s: REST :8080 + gRPC :9090", instanceID)
	err = discovery.Run(discovery.Config{
		Name:    "inscripciones",
		Handler: mux,
		Ping:    db.Ping,
		RegisterGRPC: func(s *grpc.Server) {
			pb.RegisterInscripcionesServiceServer(s, &grpcserver.Server{
				Model:   inscripcionModel,
				Service: inscripcionService,
			})
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
