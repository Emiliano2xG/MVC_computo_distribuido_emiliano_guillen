// Package clients tiene los clientes gRPC con los que inscripciones habla con
// los otros servicios. Todas las llamadas salen hacia el load balancer
// (nginx, puerto 9000), que las enruta por el nombre del servicio gRPC hacia
// una replica healthy segun Consul. Este servicio no conoce ninguna IP.
package clients

import (
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	alumnospb "horarios/pb/alumnos"
	materiaspb "horarios/pb/materias"
)

type Clients struct {
	Alumnos  alumnospb.AlumnosServiceClient
	Materias materiaspb.MateriasServiceClient
	conn     *grpc.ClientConn
}

// New abre la conexion (perezosa) hacia el gateway gRPC.
// Direccion: GRPC_GATEWAY_ADDR, por defecto load_balancer:9000.
func New() (*Clients, error) {
	addr := os.Getenv("GRPC_GATEWAY_ADDR")
	if addr == "" {
		addr = "load_balancer:9000"
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &Clients{
		Alumnos:  alumnospb.NewAlumnosServiceClient(conn),
		Materias: materiaspb.NewMateriasServiceClient(conn),
		conn:     conn,
	}, nil
}

func (c *Clients) Close() error { return c.conn.Close() }
