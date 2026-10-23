package discovery

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metricas en formato Prometheus. Se exponen en GET /metrics de cada replica
// (puerto 8080) y Prometheus las recoge descubriendo las replicas en Consul.
//
// Ademas de estas, el registro por defecto trae las metricas del runtime de
// Go (goroutines, memoria, GC) y del proceso (CPU, descriptores abiertos).
var (
	httpPeticiones = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_peticiones_total",
		Help: "Peticiones HTTP atendidas, por metodo, ruta y codigo de respuesta.",
	}, []string{"metodo", "ruta", "codigo"})

	httpDuracion = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_duracion_segundos",
		Help:    "Duracion de las peticiones HTTP.",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
	}, []string{"metodo", "ruta"})

	httpEnVuelo = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "http_peticiones_en_vuelo",
		Help: "Peticiones HTTP que se estan atendiendo ahora mismo.",
	})

	grpcLlamadas = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "grpc_llamadas_total",
		Help: "Llamadas gRPC atendidas, por metodo y codigo gRPC.",
	}, []string{"metodo", "codigo"})

	grpcDuracion = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "grpc_duracion_segundos",
		Help:    "Duracion de las llamadas gRPC.",
		Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5},
	}, []string{"metodo"})
)

// rutaMetrica reduce la ruta a su primer segmento (/inscripciones/117 ->
// /inscripciones). Si se usara la ruta completa cada id crearia una serie
// nueva y Prometheus acabaria con millones de series ("explosion de
// cardinalidad").
func rutaMetrica(path string) string {
	p := strings.TrimPrefix(path, "/")
	if i := strings.IndexByte(p, '/'); i >= 0 {
		p = p[:i]
	}
	return "/" + p
}

// estadoWriter recuerda el codigo de respuesta que escribio el handler.
type estadoWriter struct {
	http.ResponseWriter
	codigo int
}

func (w *estadoWriter) WriteHeader(c int) {
	w.codigo = c
	w.ResponseWriter.WriteHeader(c)
}

// Instrumentar mide cada peticion HTTP del handler dado. Las rutas internas
// (/health, /heartbeat) se omiten para que los health checks de Consul cada
// 5 s no tapen el trafico real.
func Instrumentar(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" || r.URL.Path == "/heartbeat" {
			next.ServeHTTP(w, r)
			return
		}
		httpEnVuelo.Inc()
		defer httpEnVuelo.Dec()

		inicio := time.Now()
		ew := &estadoWriter{ResponseWriter: w, codigo: http.StatusOK}
		next.ServeHTTP(ew, r)

		ruta := rutaMetrica(r.URL.Path)
		httpPeticiones.WithLabelValues(r.Method, ruta, strconv.Itoa(ew.codigo)).Inc()
		httpDuracion.WithLabelValues(r.Method, ruta).Observe(time.Since(inicio).Seconds())
	})
}

// ConMetricas devuelve un handler que sirve GET /metrics y deja pasar todo lo
// demas (medido) al handler del servicio.
func ConMetricas(servicio http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/", Instrumentar(servicio))
	return mux
}

func registrarGRPC(metodo, codigo string, dur time.Duration) {
	grpcLlamadas.WithLabelValues(metodo, codigo).Inc()
	grpcDuracion.WithLabelValues(metodo).Observe(dur.Seconds())
}
