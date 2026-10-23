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
// REST (HTTP) + gRPC (opcional) + registro/baja en Consul.
type Config struct {
	Name    string // nombre del servicio: "materias", "alumnos", "inscripciones"
	Handler http.Handler
	Ping    func() error // health: normalmente db.Ping

	// Registra las implementaciones gRPC (pb.RegisterXxxServer(s, impl)).
	// Si es nil el servicio solo expone REST (por ejemplo notificaciones).
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
		dur := time.Since(start)
		registrarGRPC(info.FullMethod, status.Code(err).String(), dur)
		log.Printf("grpc %s code=%s dur=%s", info.FullMethod, status.Code(err), dur.Round(time.Microsecond))
		return resp, err
	}
}

// Run levanta HTTP (:SERVICE_PORT, 8080) y, si hay RegisterGRPC, gRPC
// (:GRPC_PORT, 9090). Registra todo en Consul y, al recibir SIGTERM/SIGINT,
// da de baja antes de cerrar.
//
// En Consul quedan por replica:
//
//	<name>        (HTTP, check GET /health)
//	<name>-grpc   (gRPC, check grpc.health.v1)  solo si hay gRPC
func Run(cfg Config) error {
	id := InstanceID(cfg.Name)
	httpPort := envPort("SERVICE_PORT", 8080)
	grpcPort := envPort("GRPC_PORT", 9090)
	httpAddr := fmt.Sprintf(":%d", httpPort)
	grpcAddr := fmt.Sprintf(":%d", grpcPort)
	conGRPC := cfg.RegisterGRPC != nil

	// Todos los servicios exponen /metrics y miden sus peticiones sin tocar su codigo.
	httpSrv := &http.Server{Addr: httpAddr, Handler: ConMetricas(cfg.Handler)}
	errCh := make(chan error, 2)

	var (
		grpcSrv *grpc.Server
		hs      *health.Server
	)
	if conGRPC {
		grpcSrv = NewGRPCServer(id)
		hs = health.NewServer()
		healthpb.RegisterHealthServer(grpcSrv, hs)
		cfg.RegisterGRPC(grpcSrv)
		reflection.Register(grpcSrv)

		lis, err := net.Listen("tcp", grpcAddr)
		if err != nil {
			return fmt.Errorf("grpc listen %s: %w", grpcAddr, err)
		}
		go func() { errCh <- grpcSrv.Serve(lis) }()
	}
	go func() { errCh <- httpSrv.ListenAndServe() }()

	if err := waitReady(httpAddr, 5*time.Second); err != nil {
		return err
	}

	// El estado del health gRPC sigue a la base de datos.
	stopHealth := make(chan struct{})
	if conGRPC {
		go watchHealth(hs, cfg.Ping, stopHealth)
	}

	httpReg, err := Register(Spec{Name: cfg.Name, ID: id, Port: httpPort})
	if err != nil {
		return err
	}
	var grpcReg *Registration
	if conGRPC {
		grpcReg, err = Register(Spec{Name: cfg.Name + "-grpc", ID: id + "-grpc", Port: grpcPort, GRPC: true})
		if err != nil {
			httpReg.Deregister()
			return err
		}
		log.Printf("%s: http %s y grpc %s listos", id, httpAddr, grpcAddr)
	} else {
		log.Printf("%s: http %s listo", id, httpAddr)
	}

	var runErr error
	select {
	case runErr = <-errCh:
	case <-waitSignal():
	}

	// Primero la baja en Consul, luego se cierran los servidores.
	httpReg.Deregister()
	grpcReg.Deregister()
	close(stopHealth)

	ctx, cancel := shutdownContext()
	defer cancel()
	if conGRPC {
		hs.Shutdown()
		stopped := make(chan struct{})
		go func() { grpcSrv.GracefulStop(); close(stopped) }()
		select {
		case <-stopped:
		case <-ctx.Done():
			grpcSrv.Stop()
		}
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
