package grpcserver

import (
	"context"
	"database/sql"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"materias/models"

	pb "horarios/pb/materias"
)

// Server implementa horarios.materias.v1.MateriasService sobre el mismo modelo
// que usa el REST.
type Server struct {
	pb.UnimplementedMateriasServiceServer
	Model *models.MateriaModel
}

func aProto(m models.Materia) *pb.Materia {
	return &pb.Materia{
		Id:        int32(m.ID),
		Clave:     m.Clave,
		Nombre:    m.Nombre,
		Profesor:  m.Profesor,
		Cupo:      int32(m.Cupo),
		Inscritos: int32(m.Inscritos),
		Horario:   m.Horario,
		Dias:      m.Dias,
		Semestre:  int32(m.Semestre),
	}
}

func (s *Server) ListarMaterias(ctx context.Context, req *pb.ListarMateriasRequest) (*pb.ListarMateriasResponse, error) {
	var list []models.Materia
	var err error
	if req.GetCarreraId() > 0 {
		list, err = s.Model.GetByCarreraID(int(req.GetCarreraId()))
	} else {
		list, err = s.Model.GetAll()
	}
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	out := &pb.ListarMateriasResponse{Materias: make([]*pb.Materia, 0, len(list))}
	for _, m := range list {
		out.Materias = append(out.Materias, aProto(m))
	}
	return out, nil
}

func (s *Server) GetMateria(ctx context.Context, req *pb.GetMateriaRequest) (*pb.Materia, error) {
	m, err := s.Model.GetByID(int(req.GetId()))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, status.Errorf(codes.NotFound, "la materia %d no existe", req.GetId())
	}
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return aProto(*m), nil
}

func (s *Server) MateriaPerteneceACarrera(ctx context.Context, req *pb.PerteneceRequest) (*pb.PerteneceResponse, error) {
	ok, err := s.Model.PerteneceACarrera(int(req.GetMateriaId()), int(req.GetCarreraId()))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.PerteneceResponse{Pertenece: ok}, nil
}
