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
	HTTP                           string `json:"HTTP"`
	Interval                       string `json:"Interval"`
	Timeout                        string `json:"Timeout"`
	DeregisterCriticalServiceAfter string `json:"DeregisterCriticalServiceAfter"`
	Status                         string `json:"Status"`
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

func servicePort() int {
	if v := os.Getenv("SERVICE_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			return p
		}
	}
	return 8080
}

func instanceID(serviceName string) string {
	if v := os.Getenv("INSTANCE_ID"); v != "" {
		return v
	}
	host, _ := os.Hostname()
	return serviceName + "-" + host
}

func Register(serviceName string) (*Registration, error) {
	id := instanceID(serviceName)
	address, err := serviceAddress()
	if err != nil {
		return nil, fmt.Errorf("consul: hostname: %w", err)
	}
	port := servicePort()
	base := consulBaseURL()

	payload := servicePayload{
		ID:      id,
		Name:    serviceName,
		Address: address,
		Port:    port,
		Tags:    []string{serviceName},
		Check: &check{
			HTTP:                           fmt.Sprintf("http://%s:%d/health", address, port),
			Interval:                       "5s",
			Timeout:                        "2s",
			DeregisterCriticalServiceAfter: "30s",
			Status:                         "passing",
		},
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
			log.Printf("consul: registrado %s id=%s addr=%s:%d", serviceName, id, address, port)
			return &Registration{baseURL: base, serviceID: id, client: client}, nil
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
	return nil, fmt.Errorf("consul: no se pudo registrar %s: %w", id, lastErr)
}

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

func ListenAndServe(addr, serviceName string, handler http.Handler) error {
	server := &http.Server{Addr: addr, Handler: handler}

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	if err := waitReady(addr, 5*time.Second); err != nil {
		server.Close()
		return err
	}

	reg, err := Register(serviceName)
	if err != nil {
		server.Close()
		return err
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		reg.Deregister()
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	case <-sig:
		reg.Deregister()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(ctx)
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
