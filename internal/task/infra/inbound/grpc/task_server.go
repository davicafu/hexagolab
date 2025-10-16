package grpc

import (
	"context"

	"github.com/davicafu/hexagolab/internal/task/application"
	"github.com/google/uuid"

	// Importa el código generado por protoc
	pb "github.com/davicafu/hexagolab/gen/go/task"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GrpcTaskServer implementa la interfaz generada por gRPC.
type GrpcTaskServer struct {
	// Es necesario para la compatibilidad hacia adelante de gRPC.
	pb.UnsafeTaskServiceServer
	service *application.TaskService
}

func NewGrpcTaskServer(service *application.TaskService) *GrpcTaskServer {
	return &GrpcTaskServer{service: service}
}

// CreateTask es la implementación del RPC.
func (s *GrpcTaskServer) CreateTask(ctx context.Context, req *pb.CreateTaskRequest) (*pb.CreateTaskResponse, error) {
	assigneeID, err := uuid.Parse(req.GetAssigneeId())
	if err != nil {
		// gRPC tiene su propio sistema de errores detallados
		return nil, status.Errorf(codes.InvalidArgument, "invalid assignee_id format")
	}

	// 1. Llama a tu lógica de aplicación (no cambia nada aquí)
	task, err := s.service.CreateTask(ctx, req.GetTitle(), req.GetDescription(), assigneeID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not create task: %v", err)
	}

	// 2. Convierte la respuesta de tu dominio al formato de Protobuf
	return &pb.CreateTaskResponse{
		Id:     task.ID.String(),
		Title:  task.Title,
		Status: string(task.Status),
	}, nil
}
