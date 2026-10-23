package grpcserver

import (
	"context"
	"database/sql"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"inscripciones/models"
	"inscripciones/service"

	pb "horarios/pb/inscripciones"
)

// Server implementa horarios.inscripciones.v1.InscripcionesService.
type Server struct {
	pb.UnimplementedInscripcionesServiceServer
	Model   *models.InscripcionModel
	Service *service.InscripcionService
}

func aProto(i models.Inscripcion) *pb.Inscripcion {
	return &pb.Inscripcion{
		Id:        int32(i.ID),
		AlumnoId:  int32(i.AlumnoID),
		MateriaId: int32(i.MateriaID),
		Estado:    i.Estado,
		Materia:   i.Materia,
		Clave:     i.Clave,
		Profesor:  i.Profesor,
		Horario:   i.Horario,
		Dias:      i.Dias,
	}
}

func (s *Server) ListarPorAlumno(ctx context.Context, req *pb.ListarPorAlumnoRequest) (*pb.ListarPorAlumnoResponse, error) {
	list, err := s.Model.GetByAlumnoID(int(req.GetAlumnoId()))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	out := &pb.ListarPorAlumnoResponse{Inscripciones: make([]*pb.Inscripcion, 0, len(list))}
	for _, i := range list {
		out.Inscripciones = append(out.Inscripciones, aProto(i))
	}
	return out, nil
}

func (s *Server) Inscribir(ctx context.Context, req *pb.InscribirRequest) (*pb.Inscripcion, error) {
	item, err := s.Service.Inscribir(ctx, int(req.GetAlumnoId()), int(req.GetMateriaId()))
	switch {
	case err == nil:
		return aProto(*item), nil
	case errors.Is(err, models.ErrAlumnoNoExiste), errors.Is(err, models.ErrMateriaNoExiste):
		return nil, status.Error(codes.NotFound, err.Error())
	case errors.Is(err, models.ErrOtraCarrera):
		return nil, status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, models.ErrYaInscrito):
		return nil, status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, models.ErrEmpalme):
		return nil, status.Error(codes.Aborted, err.Error())
	case errors.Is(err, models.ErrSinCupo), errors.Is(err, models.ErrYaCursada):
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, models.ErrServicioNoDisponible):
		return nil, status.Error(codes.Unavailable, err.Error())
	default:
		return nil, status.Error(codes.Internal, err.Error())
	}
}

func (s *Server) DarDeBaja(ctx context.Context, req *pb.DarDeBajaRequest) (*pb.DarDeBajaResponse, error) {
	err := s.Service.DarDeBaja(ctx, int(req.GetId()))
	switch {
	case err == nil:
		return &pb.DarDeBajaResponse{}, nil
	case errors.Is(err, sql.ErrNoRows):
		return nil, status.Errorf(codes.NotFound, "la inscripcion %d no existe", req.GetId())
	case errors.Is(err, models.ErrYaCursada):
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	default:
		return nil, status.Error(codes.Internal, err.Error())
	}
}
