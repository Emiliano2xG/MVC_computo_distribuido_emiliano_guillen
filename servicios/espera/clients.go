package main

import (
	"context"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	alumnospb "horarios/pb/alumnos"
	inscpb "horarios/pb/inscripciones"
	materiaspb "horarios/pb/materias"
)

// Clients son los clientes gRPC hacia los otros servicios. Igual que en
// inscripciones, todo sale por el gateway nginx (:9000): este servicio no
// conoce ninguna IP ni replica.
type Clients struct {
	Alumnos       alumnospb.AlumnosServiceClient
	Materias      materiaspb.MateriasServiceClient
	Inscripciones inscpb.InscripcionesServiceClient
	conn          *grpc.ClientConn
	caller        string
}

func NewClients(caller string) (*Clients, error) {
	addr := os.Getenv("GRPC_GATEWAY_ADDR")
	if addr == "" {
		addr = "load_balancer:9000"
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &Clients{
		Alumnos:       alumnospb.NewAlumnosServiceClient(conn),
		Materias:      materiaspb.NewMateriasServiceClient(conn),
		Inscripciones: inscpb.NewInscripcionesServiceClient(conn),
		conn:          conn,
		caller:        caller,
	}, nil
}

func (c *Clients) Close() error { return c.conn.Close() }

// Llamada prepara el contexto de una llamada saliente: timeout de 3 s y el
// header x-caller (queda en los logs de quien responde).
func (c *Clients) Llamada(ctx context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	return metadata.AppendToOutgoingContext(ctx, "x-caller", c.caller), cancel
}
