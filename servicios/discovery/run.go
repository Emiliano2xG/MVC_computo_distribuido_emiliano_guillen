package discovery

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

// Config junta todo lo que un backend necesita para vivir en el sistema:
// REST (HTTP) + gRPC + registro/baja en Consul.
type Config struct {
	Name    string // nombre del servicio: "materias", "alumnos", "inscripciones"
	Handler http.Handler
	Ping    func() error // health: normalmente db.Ping

	// Registra las implementaciones gRPC (pb.RegisterXxxServer(s, impl)).
	RegisterGRPC func(s *grpc.Server)
}

// NewGRPCServer crea el servidor gRPC con el interceptor que loguea cada
// llamada y agrega el header x-instance-id (sirve para ver que replica
// atendio cuando se pasa por el load balancer).
func NewGRPCServer(instanceID string) *grpc.Server {
	return grpc.NewServer(grpc.ChainUnaryInterceptor(unaryInterceptor(instanceID)))
}

func unaryInterceptor(instanceID string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
		start := time.Now()
		_ = grpc.SetHeader(ctx, metadata.Pairs("x-instance-id", instanceID))
		resp, err := h(ctx, req)
		log.Printf("grpc %s code=%s dur=%s", info.FullMethod, status.Code(err), time.Since(start).Round(time.Microsecond))
		return resp, err
	}
}

// Run levanta HTTP (:SERVICE_PORT, 8080) y gRPC (:GRPC_PORT, 9090), registra
// ambos en Consul, y al recibir SIGTERM/SIGINT los da de baja antes de cerrar.
//
// En Consul quedan dos servicios por replica:
//
//	<name>        (HTTP, check GET /health)
//	<name>-grpc   (gRPC, check grpc.health.v1)
func Run(cfg Config) error {
	id := InstanceID(cfg.Name)
	httpPort := envPort("SERVICE_PORT", 8080)
	grpcPort := envPort("GRPC_PORT", 9090)
	httpAddr := fmt.Sprintf(":%d", httpPort)
	grpcAddr := fmt.Sprintf(":%d", grpcPort)

	httpSrv := &http.Server{Addr: httpAddr, Handler: cfg.Handler}

	grpcSrv := NewGRPCServer(id)
	hs := health.NewServer()
	healthpb.RegisterHealthServer(grpcSrv, hs)
	if cfg.RegisterGRPC != nil {
		cfg.RegisterGRPC(grpcSrv)
	}
	reflection.Register(grpcSrv)

	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		return fmt.Errorf("grpc listen %s: %w", grpcAddr, err)
	}

	errCh := make(chan error, 2)
	go func() { errCh <- httpSrv.ListenAndServe() }()
	go func() { errCh <- grpcSrv.Serve(lis) }()

	if err := waitReady(httpAddr, 5*time.Second); err != nil {
		return err
	}

	// El estado del health gRPC sigue a la base de datos.
	stopHealth := make(chan struct{})
	go watchHealth(hs, cfg.Ping, stopHealth)

	httpReg, err := Register(Spec{Name: cfg.Name, ID: id, Port: httpPort})
	if err != nil {
		return err
	}
	grpcReg, err := Register(Spec{Name: cfg.Name + "-grpc", ID: id + "-grpc", Port: grpcPort, GRPC: true})
	if err != nil {
		httpReg.Deregister()
		return err
	}
	log.Printf("%s: http %s y grpc %s listos", id, httpAddr, grpcAddr)

	var runErr error
	select {
	case runErr = <-errCh:
	case <-waitSignal():
	}

	// Primero la baja en Consul, luego se cierran los servidores.
	httpReg.Deregister()
	grpcReg.Deregister()
	close(stopHealth)
	hs.Shutdown()

	ctx, cancel := shutdownContext()
	defer cancel()
	stopped := make(chan struct{})
	go func() { grpcSrv.GracefulStop(); close(stopped) }()
	select {
	case <-stopped:
	case <-ctx.Done():
		grpcSrv.Stop()
	}
	_ = httpSrv.Shutdown(ctx)

	if runErr == http.ErrServerClosed || runErr == grpc.ErrServerStopped {
		return nil
	}
	return runErr
}

func watchHealth(hs *health.Server, ping func() error, stop <-chan struct{}) {
	set := func() {
		st := healthpb.HealthCheckResponse_SERVING
		if ping != nil && ping() != nil {
			st = healthpb.HealthCheckResponse_NOT_SERVING
		}
		hs.SetServingStatus("", st)
	}
	set()
	t := time.NewTicker(3 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			set()
		case <-stop:
			return
		}
	}
}
