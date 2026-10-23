package mensajeria

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
)

func brokers() []string {
	v := strings.TrimSpace(os.Getenv("KAFKA_BROKERS"))
	if v == "" {
		return nil
	}
	return strings.Split(v, ",")
}

// ---------------------------------------------------------------------------
// Productor
// ---------------------------------------------------------------------------

// Productor publica eventos en Kafka sin bloquear a quien lo llama: Publicar
// regresa al instante y el envio (con reintentos) ocurre en segundo plano.
// Asi una peticion REST/gRPC no depende de que Kafka este arriba.
//
// Si KAFKA_BROKERS esta vacio queda deshabilitado (no hace nada).
type Productor struct {
	w      *kafka.Writer
	origen string
	wg     sync.WaitGroup
}

// NuevoProductor lee KAFKA_BROKERS e INSTANCE_ID del entorno.
func NuevoProductor() *Productor {
	p := &Productor{origen: os.Getenv("INSTANCE_ID")}
	b := brokers()
	if len(b) == 0 {
		log.Printf("kafka: KAFKA_BROKERS vacio, productor deshabilitado")
		return p
	}
	p.w = &kafka.Writer{
		Addr:                   kafka.TCP(b...),
		Balancer:               &kafka.Hash{}, // misma llave -> misma particion
		RequiredAcks:           kafka.RequireAll,
		AllowAutoTopicCreation: false, // los topicos los crea kafka_init
		WriteTimeout:           5 * time.Second,
		BatchTimeout:           10 * time.Millisecond,
		MaxAttempts:            5,
	}
	log.Printf("kafka: productor listo (%s)", strings.Join(b, ","))
	return p
}

// Publicar manda un evento al topico con la llave dada. No bloquea ni
// devuelve error: si falla, queda en el log.
func (p *Productor) Publicar(topic, key, tipo string, datos any) {
	if p == nil || p.w == nil {
		return
	}
	ev, err := NuevoEvento(tipo, p.origen, datos)
	if err != nil {
		log.Printf("kafka: no se pudo armar el evento %s: %v", tipo, err)
		return
	}
	value, err := json.Marshal(ev)
	if err != nil {
		log.Printf("kafka: no se pudo serializar %s: %v", tipo, err)
		return
	}
	msg := kafka.Message{
		Topic:   topic,
		Key:     []byte(key),
		Value:   value,
		Headers: []kafka.Header{{Key: "tipo", Value: []byte(tipo)}},
	}
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := p.w.WriteMessages(ctx, msg); err != nil {
			kafkaPublicados.WithLabelValues(topic, tipo, "error").Inc()
			log.Printf("kafka: ERROR publicando %s en %s (key=%s): %v", tipo, topic, key, err)
			return
		}
		kafkaPublicados.WithLabelValues(topic, tipo, "ok").Inc()
		log.Printf("kafka: publicado %s en %s (key=%s id=%s)", tipo, topic, key, ev.ID)
	}()
}

// Cerrar espera los envios pendientes y cierra la conexion.
func (p *Productor) Cerrar() {
	if p == nil || p.w == nil {
		return
	}
	p.wg.Wait()
	_ = p.w.Close()
}

// ---------------------------------------------------------------------------
// Consumidor
// ---------------------------------------------------------------------------

// Handler procesa un evento. Si devuelve error se reintenta (hasta 3 veces).
type Handler func(ctx context.Context, topic string, ev Evento) error

// Consumir se suscribe a los topicos como parte del grupo groupID y llama a h
// por cada mensaje hasta que ctx se cancele. Bloquea: llamarlo en una goroutine.
//
// Semantica "al menos una vez": el offset se confirma despues de procesar, asi
// que un evento puede llegar repetido (los handlers deben tolerarlo; el id del
// evento sirve para detectarlo). Las replicas de un servicio comparten el
// groupID y Kafka reparte las particiones entre ellas.
//
// Si KAFKA_BROKERS esta vacio no hace nada.
func Consumir(ctx context.Context, groupID string, topics []string, h Handler) {
	b := brokers()
	if len(b) == 0 {
		log.Printf("kafka: KAFKA_BROKERS vacio, consumidor %s deshabilitado", groupID)
		return
	}
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     b,
		GroupID:     groupID,
		GroupTopics: topics,
		MinBytes:    1,
		MaxBytes:    1 << 20,
		MaxWait:     500 * time.Millisecond,
		StartOffset: kafka.FirstOffset, // grupo nuevo: lee desde el principio
	})
	defer r.Close()
	log.Printf("kafka: consumidor group=%s topicos=%s", groupID, strings.Join(topics, ","))

	for {
		m, err := r.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("kafka: group=%s error leyendo: %v (reintento en 2s)", groupID, err)
			select {
			case <-time.After(2 * time.Second):
			case <-ctx.Done():
				return
			}
			continue
		}

		var ev Evento
		if err := json.Unmarshal(m.Value, &ev); err != nil {
			log.Printf("kafka: group=%s mensaje invalido en %s[%d]@%d, se descarta: %v", groupID, m.Topic, m.Partition, m.Offset, err)
		} else {
			procesar(ctx, groupID, h, m.Topic, ev)
		}
		if err := r.CommitMessages(ctx, m); err != nil && ctx.Err() == nil {
			log.Printf("kafka: group=%s error confirmando offset: %v", groupID, err)
		}
	}
}

func procesar(ctx context.Context, groupID string, h Handler, topic string, ev Evento) {
	var err error
	for intento := 1; intento <= 3; intento++ {
		if err = h(ctx, topic, ev); err == nil {
			kafkaConsumidos.WithLabelValues(groupID, topic, ev.Tipo, "ok").Inc()
			return
		}
		log.Printf("kafka: group=%s fallo %s id=%s (intento %d/3): %v", groupID, ev.Tipo, ev.ID, intento, err)
		select {
		case <-time.After(time.Duration(intento) * 500 * time.Millisecond):
		case <-ctx.Done():
			return
		}
	}
	kafkaConsumidos.WithLabelValues(groupID, topic, ev.Tipo, "descartado").Inc()
	log.Printf("kafka: group=%s DESCARTADO %s id=%s tras 3 intentos: %v", groupID, ev.Tipo, ev.ID, err)
}
