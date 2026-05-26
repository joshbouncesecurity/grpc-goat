package main

import (
	"context"
	"log"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/reflection"

	pb "grpc-goat/labs/grpc-001-reflection-enabled/server/proto"
)

// unaryInterceptor logs client IP addresses for all requests
func unaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	// Extract client IP from context
	peer, ok := peer.FromContext(ctx)
	if ok {
		log.Printf("Request from %s to %s", peer.Addr, info.FullMethod)
	}

	// Call the actual handler
	return handler(ctx, req)
}

type serviceDiscoveryServer struct {
	pb.UnimplementedServiceDiscoveryServer
}

func newServiceDiscoveryServer() *serviceDiscoveryServer {
	return &serviceDiscoveryServer{}
}

func (s *serviceDiscoveryServer) ListServices(ctx context.Context, req *pb.ListServicesRequest) (*pb.ListServicesResponse, error) {

	services := []*pb.ServiceInfo{
		{Name: "user-api", Endpoint: "user-api:8080", Type: "public"},
		{Name: "payment-service", Endpoint: "payment:8081", Type: "public"},
	}

	return &pb.ListServicesResponse{
		Services: services,
	}, nil
}

func (s *serviceDiscoveryServer) AdminListAllServices(ctx context.Context, req *pb.AdminListAllServicesRequest) (*pb.AdminListAllServicesResponse, error) {
	publicServices := []*pb.ServiceInfo{
		{Name: "user-api", Endpoint: "user-api:8080", Type: "public"},
		{Name: "payment-service", Endpoint: "payment:8081", Type: "public"},
	}

	adminServices := []*pb.ServiceInfo{
		{Name: "admin-panel", Endpoint: "admin:9090", Type: "admin"},
		{Name: "database-admin", Endpoint: "db-admin:9091", Type: "admin"},
		{Name: "user-management", Endpoint: "user-mgmt:9092", Type: "admin"},
	}

	return &pb.AdminListAllServicesResponse{
		PublicServices: publicServices,
		AdminServices:  adminServices,
		Flag:           "GRPC_GOAT{reflection_exposes_hidden_admin_methods}",
	}, nil
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8001"
	}
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	s := grpc.NewServer(grpc.UnaryInterceptor(unaryInterceptor))

	pb.RegisterServiceDiscoveryServer(s, newServiceDiscoveryServer())

	reflection.Register(s)

	log.Printf("gRPC server starting on port %s...", port)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
