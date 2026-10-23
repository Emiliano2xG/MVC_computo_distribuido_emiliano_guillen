package mensajeria

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metricas de Kafka. Se registran en el registro por defecto de Prometheus,
// el mismo que sirve GET /metrics en cada servicio (ver horarios/discovery).
var (
	kafkaPublicados = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "kafka_eventos_publicados_total",
		Help: "Eventos publicados en Kafka, por topico, tipo y resultado (ok / error).",
	}, []string{"topico", "tipo", "resultado"})

	kafkaConsumidos = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "kafka_eventos_consumidos_total",
		Help: "Eventos leidos de Kafka, por grupo, topico, tipo y resultado (ok / descartado).",
	}, []string{"grupo", "topico", "tipo", "resultado"})
)
