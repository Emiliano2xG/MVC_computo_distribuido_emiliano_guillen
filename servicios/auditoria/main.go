// Servicio de auditoria: bitacora permanente de todo lo que pasa en el sistema.
//
// Consume los tres topicos de Kafka (alumnos, materias e inscripciones) y
// guarda cada evento en Postgres. Es el "libro de contabilidad" del sistema:
// quien hizo que, cuando y que replica lo atendio.
//
// Diferencia con notificaciones: notificaciones guarda las ultimas 200 en
// memoria y se pierde al reiniciar; auditoria guarda todo en la base y se
// puede consultar despues. Corre con 2 replicas en el mismo grupo de Kafka:
// las particiones se reparten entre ellas y, como el id del evento es llave
// primaria, una entrega repetida no duplica el registro.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"horarios/discovery"
	"horarios/mensajeria"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	_ "github.com/lib/pq"
)

const grupo = "auditoria"

var guardados = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "auditoria_eventos_guardados_total",
	Help: "Eventos escritos en la bitacora, por tipo y resultado (nuevo / repetido).",
}, []string{"tipo", "resultado"})

func main() {
	instanceID := os.Getenv("INSTANCE_ID")
	if instanceID == "" {
		instanceID = "auditoria"
	}

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_NAME"))
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Failed to configure database:", err)
	}
	defer db.Close()

	var pingErr error
	for i := 0; i < 15; i++ {
		if pingErr = db.Ping(); pingErr == nil {
			break
		}
		time.Sleep(time.Second)
	}
	if pingErr != nil {
		log.Fatal("Failed to ping database:", pingErr)
	}

	store := &Store{DB: db, Replica: instanceID}
	if err := store.Migrar(context.Background()); err != nil {
		log.Fatal("migracion de auditoria_eventos: ", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mensajeria.Consumir(ctx, grupo, mensajeria.Topics, manejador(store))

	mux := http.NewServeMux()
	mux.HandleFunc("/auditoria", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		limite, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limite <= 0 {
			limite = 50
		}
		if limite > 500 {
			limite = 500
		}
		lista, err := store.Listar(r.Context(), r.URL.Query().Get("tipo"), r.URL.Query().Get("topico"), limite)
		responder(w, instanceID, lista, err)
	})
	mux.HandleFunc("/auditoria/resumen", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		res, err := store.Resumen(r.Context())
		responder(w, instanceID, res, err)
	})
	mux.HandleFunc("/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("backend alive"))
	})
	mux.HandleFunc("/health", discovery.HealthHandler(db.Ping))

	log.Printf("backend %s: REST :8080 + consumidor Kafka (grupo %s) + Postgres", instanceID, grupo)
	if err := discovery.Run(discovery.Config{Name: "auditoria", Handler: mux, Ping: db.Ping}); err != nil {
		log.Fatal(err)
	}
}

func responder(w http.ResponseWriter, instancia string, v any, err error) {
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Instance-Id", instancia)
	json.NewEncoder(w).Encode(v)
}

// manejador guarda cada evento. A diferencia de notificaciones, aqui perder un
// evento es inaceptable, asi que si la base falla NO se descarta tras 3
// intentos: se reintenta con espera creciente (tope 10 s) hasta que la base
// vuelva. Mientras tanto el consumo de esa particion queda detenido y el
// "lag" en Kafka crece, lo que es justo lo que queremos que se vea.
func manejador(s *Store) mensajeria.Handler {
	return func(ctx context.Context, topico string, ev mensajeria.Evento) error {
		espera := time.Second
		for {
			nuevo, err := s.Guardar(ctx, topico, ev)
			if err == nil {
				res := "nuevo"
				if !nuevo {
					res = "repetido"
					log.Printf("auditoria: evento repetido %s (id=%s), ya estaba guardado", ev.Tipo, ev.ID)
				} else {
					log.Printf("auditoria: guardado %s id=%s origen=%s (%s)", ev.Tipo, ev.ID, ev.Origen, topico)
				}
				guardados.WithLabelValues(ev.Tipo, res).Inc()
				return nil
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			log.Printf("auditoria: no se pudo guardar %s id=%s: %v (reintento en %s)", ev.Tipo, ev.ID, err, espera)
			select {
			case <-time.After(espera):
			case <-ctx.Done():
				return ctx.Err()
			}
			if espera < 10*time.Second {
				espera *= 2
			}
		}
	}
}
