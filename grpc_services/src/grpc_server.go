package main

import (
	"crypto/tls"
	"flag"
	"net"
	"os"
	"sync"

	echo "github.com/salrashid123/grpc_wireformat/grpc_services/src/echo"

	"log"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

var (
	insecure = flag.Bool("insecure", false, "start without tls")
	grpcport = flag.String("grpcport", ":50051", "grpcport")
	tlsCert  = flag.String("tlsCert", "certs/localhost.crt", "TLS Cert")
	tlsKey   = flag.String("tlsKey", "certs/localhost.key", "TLS Key")
	tlsCA    = flag.String("tlsCA", "certs/tls-ca-chain.pem", "TLS CA")
)

const (
	address string = ":50051"
)

type Server struct {
	mu sync.Mutex
	// statusMap stores the serving status of the services this Server monitors.
	statusMap map[string]healthpb.HealthCheckResponse_ServingStatus
	// Embed the unimplemented server
	echo.UnimplementedEchoServerServer
}

// NewServer returns a new Server.
func NewServer() *Server {
	return &Server{
		statusMap: make(map[string]healthpb.HealthCheckResponse_ServingStatus),
	}
}

func (s *Server) SayHello(ctx context.Context, in *echo.EchoRequest) (*echo.EchoReply, error) {
	mname := ""
	m := in.MiddleName
	if m != nil {
		mname = m.Name
	}
	log.Printf("Got rpc: --> %s %s %s \n", in.FirstName, mname, in.LastName)
	return &echo.EchoReply{Message: "Hello " + in.FirstName + " " + mname + " " + in.LastName}, nil
}

func (s *Server) Check(ctx context.Context, in *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if in.Service == "" {
		// return overall status
		return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING}, nil
	}

	s.statusMap["echo.EchoServer"] = healthpb.HealthCheckResponse_SERVING

	status, ok := s.statusMap[in.Service]
	if !ok {
		return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_UNKNOWN}, grpc.Errorf(codes.NotFound, "unknown service")
	}
	return &healthpb.HealthCheckResponse{Status: status}, nil
}

func (s *Server) Watch(in *healthpb.HealthCheckRequest, srv healthpb.Health_WatchServer) error {
	return status.Error(codes.Unimplemented, "Watch is not implemented")
}

func main() {

	flag.Parse()

	if *grpcport == "" {
		log.Printf("missing -grpcport flag (:50051)")
		flag.Usage()
		os.Exit(2)
	}

	sopts := []grpc.ServerOption{grpc.MaxConcurrentStreams(10)}
	lis, err := net.Listen("tcp", *grpcport)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	if !*insecure {
		certificate, err := tls.LoadX509KeyPair(*tlsCert, *tlsKey)
		if err != nil {
			log.Fatalf("could not load server key pair: %s", err)
		}

		tlsConfig := tls.Config{
			Certificates: []tls.Certificate{certificate},
		}
		creds := credentials.NewTLS(&tlsConfig)

		sopts = append(sopts, grpc.Creds(creds))
	}

	s := grpc.NewServer(sopts...)
	srv := NewServer()
	healthpb.RegisterHealthServer(s, srv)
	echo.RegisterEchoServerServer(s, srv)
	reflection.Register(s)
	log.Println("Starting Server...")
	s.Serve(lis)

}
