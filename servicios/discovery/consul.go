package discovery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Registration es un servicio registrado en Consul. Se da de baja con Deregister.
type Registration struct {
	baseURL   string
	serviceID string
	client    *http.Client
}

type servicePayload struct {
	ID      string   `json:"ID"`
	Name    string   `json:"Name"`
	Address string   `json:"Address"`
	Port    int      `json:"Port"`
	Tags    []string `json:"Tags,omitempty"`
	Check   *check   `json:"Check"`
}

type check struct {
	HTTP                           string `json:"HTTP,omitempty"`
	GRPC                           string `json:"GRPC,omitempty"`
	GRPCUseTLS                     bool   `json:"GRPCUseTLS,omitempty"`
	Interval                       string `json:"Interval"`
	Timeout                        string `json:"Timeout"`
	DeregisterCriticalServiceAfter string `json:"DeregisterCriticalServiceAfter"`
	Status                         string `json:"Status"`
}

// Spec describe un servicio a registrar. Con GRPC=true el health check de
// Consul es el protocolo estandar grpc.health.v1 (en vez de GET /health).
type Spec struct {
	Name string
	ID   string
	Port int
	GRPC bool
}

func consulBaseURL() string {
	addr := os.Getenv("CONSUL_HTTP_ADDR")
	if addr == "" {
		addr = "consul:8500"
	}
	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		addr = "http://" + addr
	}
	return strings.TrimRight(addr, "/")
}

func serviceAddress() (string, error) {
	if v := os.Getenv("SERVICE_ADDRESS"); v != "" {
		return v, nil
	}
	return os.Hostname()
}

func envPort(name string, def int) int {
	if v := os.Getenv(name); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			return p
		}
	}
	return def
}

// InstanceID devuelve INSTANCE_ID o, si no esta, un id basado en el hostname.
func InstanceID(serviceName string) string {
	if v := os.Getenv("INSTANCE_ID"); v != "" {
		return v
	}
	host, _ := os.Hostname()
	return serviceName + "-" + host
}

// Register da de alta un servicio en el service registry de Consul con su
// health check. Reintenta hasta 12 veces (Consul puede estar arrancando).
func Register(s Spec) (*Registration, error) {
	address, err := serviceAddress()
	if err != nil {
		return nil, fmt.Errorf("consul: hostname: %w", err)
	}
	base := consulBaseURL()

	chk := &check{
		Interval:                       "5s",
		Timeout:                        "2s",
		DeregisterCriticalServiceAfter: "30s",
		Status:                         "passing",
	}
	tags := []string{s.Name}
	if s.GRPC {
		chk.GRPC = fmt.Sprintf("%s:%d", address, s.Port)
		tags = append(tags, "grpc")
	} else {
		chk.HTTP = fmt.Sprintf("http://%s:%d/health", address, s.Port)
		tags = append(tags, "http")
	}

	payload := servicePayload{
		ID:      s.ID,
		Name:    s.Name,
		Address: address,
		Port:    s.Port,
		Tags:    tags,
		Check:   chk,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 3 * time.Second}
	url := base + "/v1/agent/service/register"
	var lastErr error
	for i := 0; i < 12; i++ {
		req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			resp.Body.Close()
			log.Printf("consul: registrado %s id=%s addr=%s:%d", s.Name, s.ID, address, s.Port)
			return &Registration{baseURL: base, serviceID: s.ID, client: client}, nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("consul register status %s", resp.Status)
			resp.Body.Close()
		}
		log.Printf("consul: reintento registro (%d/12): %v", i+1, lastErr)
		time.Sleep(time.Second)
	}
	return nil, fmt.Errorf("consul: no se pudo registrar %s: %w", s.ID, lastErr)
}

// Deregister saca el servicio del registry de Consul.
func (r *Registration) Deregister() {
	if r == nil || r.client == nil {
		return
	}
	url := r.baseURL + "/v1/agent/service/deregister/" + r.serviceID
	req, err := http.NewRequest(http.MethodPut, url, nil)
	if err != nil {
		log.Printf("consul: baja %s: %v", r.serviceID, err)
		return
	}
	resp, err := r.client.Do(req)
	if err != nil {
		log.Printf("consul: baja %s: %v", r.serviceID, err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Printf("consul: dado de baja %s", r.serviceID)
		return
	}
	log.Printf("consul: baja %s status %s", r.serviceID, resp.Status)
}

// HealthHandler es el GET /health que consulta Consul (check HTTP).
func HealthHandler(ping func() error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if ping != nil {
			if err := ping(); err != nil {
				http.Error(w, "unhealthy", http.StatusServiceUnavailable)
				return
			}
		}
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}
}

func waitReady(addr string, timeout time.Duration) error {
	dial := addr
	if strings.HasPrefix(dial, ":") {
		dial = "127.0.0.1" + dial
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", dial, 200*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("timeout esperando %s", addr)
}

func waitSignal() <-chan os.Signal {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	return sig
}

func shutdownContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}
