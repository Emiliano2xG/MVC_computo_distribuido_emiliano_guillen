package grpcserver

import (
	"context"
	"database/sql"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"alumnos/models"

	pb "horarios/pb/alumnos"
)

// Server implementa horarios.alumnos.v1.AlumnosService sobre el mismo modelo
// que usa el REST.
type Server struct {
	pb.UnimplementedAlumnosServiceServer
	Model *models.AlumnoModel
}

func aProto(a *models.Alumno) *pb.Alumno {
	return &pb.Alumno{
		Id:        int32(a.ID),
		Nombre:    a.Nombre,
		Matricula: a.Matricula,
		Correo:    a.Correo,
		CarreraId: int32(a.CarreraID),
		Carrera:   a.Carrera,
	}
}

func (s *Server) GetAlumno(ctx context.Context, req *pb.GetAlumnoRequest) (*pb.Alumno, error) {
	a, err := s.Model.GetByID(int(req.GetId()))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, status.Errorf(codes.NotFound, "el alumno %d no existe", req.GetId())
	}
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return aProto(a), nil
}

func (s *Server) Login(ctx context.Context, req *pb.LoginRequest) (*pb.Alumno, error) {
	a, err := s.Model.Login(req.GetCorreo(), req.GetPassword())
	if errors.Is(err, models.ErrLogin) {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return aProto(a), nil
}

func (s *Server) ListarCarreras(ctx context.Context, req *pb.ListarCarrerasRequest) (*pb.ListarCarrerasResponse, error) {
	list, err := s.Model.GetCarreras()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	out := &pb.ListarCarrerasResponse{Carreras: make([]*pb.Carrera, 0, len(list))}
	for _, c := range list {
		out.Carreras = append(out.Carreras, &pb.Carrera{Id: int32(c.ID), Clave: c.Clave, Nombre: c.Nombre})
	}
	return out, nil
}
