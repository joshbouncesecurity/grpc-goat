package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	_ "modernc.org/sqlite"

	pb "grpc-goat/combined/server/proto"
)

func unaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	p, ok := peer.FromContext(ctx)
	if ok {
		log.Printf("Request from %s to %s", p.Addr, info.FullMethod)
	}
	return handler(ctx, req)
}

// --- Lab 001: Service Discovery (reflection enabled) ---

type serviceDiscoveryServer struct {
	pb.UnimplementedServiceDiscoveryServer
}

func (s *serviceDiscoveryServer) ListServices(ctx context.Context, req *pb.ListServicesRequest) (*pb.ListServicesResponse, error) {
	return &pb.ListServicesResponse{
		Services: []*pb.ServiceInfo{
			{Name: "[lab007] UserDirectory", Endpoint: "user-directory:8080", Type: "public"},
			{Name: "[lab008] FileProcessor", Endpoint: "file-processor:8081", Type: "public"},
		},
	}, nil
}

func (s *serviceDiscoveryServer) AdminListAllServices(ctx context.Context, req *pb.AdminListAllServicesRequest) (*pb.AdminListAllServicesResponse, error) {
	if req.AdminToken == "" {
		return nil, status.Error(codes.Unauthenticated, "admin_token required")
	}

	return &pb.AdminListAllServicesResponse{
		PublicServices: []*pb.ServiceInfo{
			{Name: "[lab007] UserDirectory", Endpoint: "user-directory:8080", Type: "public"},
			{Name: "[lab008] FileProcessor", Endpoint: "file-processor:8081", Type: "public"},
		},
		AdminServices: []*pb.ServiceInfo{
			{Name: "[lab001] ServiceDiscovery", Endpoint: "service-discovery:8082", Type: "admin"},
			{Name: "[lab009] ImagePreview", Endpoint: "image-preview:8083", Type: "admin"},
		},
		Flag: "GRPC_GOAT{reflection_exposes_hidden_admin_methods}",
	}, nil
}

// --- Lab 007: User Directory (SQL injection) ---

type userDirectoryServer struct {
	pb.UnimplementedUserDirectoryServer
	db *sql.DB
}

func newUserDirectoryServer() *userDirectoryServer {
	setupDB, err := sql.Open("sqlite", "/tmp/users.db")
	if err != nil {
		log.Fatalf("Failed to create database: %v", err)
	}
	srv := &userDirectoryServer{db: setupDB}
	srv.initDatabase()
	setupDB.Close()

	db, err := sql.Open("sqlite", "file:/tmp/users.db?mode=ro")
	if err != nil {
		log.Fatalf("Failed to open read-only database: %v", err)
	}
	srv.db = db
	return srv
}

func (s *userDirectoryServer) initDatabase() {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS users (username TEXT PRIMARY KEY, email TEXT NOT NULL, role TEXT NOT NULL)`)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}
	for _, u := range [][]string{
		{"john", "john@company.com", "user"},
		{"admin", "admin@company.com", "admin"},
		{"flag_user", "flag@company.com", "GRPC_GOAT{sql_injection_data_exfiltration}"},
	} {
		if _, err := s.db.Exec(`INSERT INTO users (username, email, role) VALUES (?, ?, ?)`, u[0], u[1], u[2]); err != nil {
			log.Printf("Failed to insert user %s: %v", u[0], err)
		}
	}
	log.Println("Database initialized")
}

func (s *userDirectoryServer) SearchUsers(ctx context.Context, req *pb.SearchUsersRequest) (*pb.SearchUsersResponse, error) {
	// VULNERABILITY: SQL injection
	query := fmt.Sprintf("SELECT username, email, role FROM users WHERE username = '%s'", req.Username)
	log.Printf("Executing SQL: %s", query)

	rows, err := s.db.Query(query)
	if err != nil {
		return &pb.SearchUsersResponse{Success: false}, nil
	}
	defer rows.Close()

	var users []*pb.UserInfo
	flag := ""
	for rows.Next() {
		var u pb.UserInfo
		if err := rows.Scan(&u.Username, &u.Email, &u.Role); err != nil {
			continue
		}
		users = append(users, &u)
		if strings.Contains(u.Role, "GRPC_GOAT") {
			flag = "GRPC_GOAT{sql_injection_data_exfiltration}"
		}
	}
	return &pb.SearchUsersResponse{Success: true, Users: users, Flag: flag}, nil
}

// --- Lab 008: File Processor (command injection) ---

type fileProcessorServer struct {
	pb.UnimplementedFileProcessorServer
}

func (s *fileProcessorServer) ListFiles(ctx context.Context, req *pb.ListFilesRequest) (*pb.ListFilesResponse, error) {
	// VULNERABILITY: command injection
	var shell, flag1, command string
	if runtime.GOOS == "windows" {
		shell, flag1 = "cmd", "/C"
		command = fmt.Sprintf("dir %s", req.Directory)
	} else {
		shell, flag1 = "sh", "-c"
		command = fmt.Sprintf("ls -la %s", req.Directory)
	}
	log.Printf("Executing command: %s", command)

	output, err := exec.Command(shell, flag1, command).Output()
	if err != nil {
		return &pb.ListFilesResponse{Success: false, Output: fmt.Sprintf("Command execution failed: %v", err)}, nil
	}
	outputStr := strings.TrimSpace(string(output))
	flag := ""
	if strings.Contains(outputStr, "GRPC_GOAT{command_injection_file_listing}") {
		flag = "GRPC_GOAT{command_injection_file_listing}"
	}
	return &pb.ListFilesResponse{Success: true, Output: outputStr, Flag: flag}, nil
}

// --- Lab 009: Image Preview (SSRF) ---

type imagePreviewServer struct {
	pb.UnimplementedImagePreviewServer
}

// startFlagServer starts a local HTTP server reachable via SSRF at http://localhost:9090/flag
func startFlagServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/flag" {
			fmt.Fprint(w, "GRPC_GOAT{ssrf_internal_service_access}")
		} else {
			fmt.Fprint(w, "Internal service - try /flag endpoint")
		}
	})
	go func() {
		if err := http.ListenAndServe("127.0.0.1:9090", mux); err != nil {
			log.Printf("Flag server error: %v", err)
		}
	}()
}

func (s *imagePreviewServer) FetchImage(ctx context.Context, req *pb.FetchImageRequest) (*pb.FetchImageResponse, error) {
	// VULNERABILITY: SSRF - no URL validation
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(req.Url)
	if err != nil {
		return &pb.FetchImageResponse{Success: false, Content: "Failed to fetch URL: " + err.Error()}, nil
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &pb.FetchImageResponse{Success: false, Content: "Failed to read response: " + err.Error()}, nil
	}
	return &pb.FetchImageResponse{Success: true, Content: string(body)}, nil
}

// --- Main ---

func main() {
	startFlagServer()
	time.Sleep(500 * time.Millisecond)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	s := grpc.NewServer(grpc.UnaryInterceptor(unaryInterceptor))

	pb.RegisterServiceDiscoveryServer(s, &serviceDiscoveryServer{})
	pb.RegisterUserDirectoryServer(s, newUserDirectoryServer())
	pb.RegisterFileProcessorServer(s, &fileProcessorServer{})
	pb.RegisterImagePreviewServer(s, &imagePreviewServer{})

	// Reflection intentionally enabled (lab 001 — now exposes all services)
	reflection.Register(s)

	log.Printf("grpc-goat combined server on port %s (labs 001, 007, 008, 009)", port)
	log.Println("Lab 009 SSRF flag server: http://localhost:9090/flag")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
