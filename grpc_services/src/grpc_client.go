package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"io/ioutil"
	"time"

	echo "github.com/salrashid123/grpc_wireformat/grpc_services/src/echo"

	"log"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

const ()

var (
	conn *grpc.ClientConn

	address         = flag.String("host", "localhost:50051", "host:port of gRPC server")
	cacert          = flag.String("cacert", "certs/tls-ca-chain.pem", "CACert for server")
	serverName      = flag.String("servername", "localhost", "SNI for server")
	skipHealthCheck = flag.Bool("skipHealthCheck", false, "Skip Initial Healthcheck")
)

func main() {

	flag.Parse()

	caCert, err := ioutil.ReadFile(*cacert)
	if err != nil {
		log.Fatalf("did not load ca: %v", err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	var conn *grpc.ClientConn

	tlsConfig := tls.Config{
		ServerName: *serverName,
		RootCAs:    caCertPool,
	}

	creds := credentials.NewTLS(&tlsConfig)

	conn, err = grpc.Dial(*address, grpc.WithTransportCredentials(creds))

	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	c := echo.NewEchoServerClient(conn)
	ctx := context.Background()

	// how to perform healthcheck request manually:
	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	if !*skipHealthCheck {
		resp, err := healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{Service: "echo.EchoServer"})
		if err != nil {
			log.Fatalf("HealthCheck failed %+v", err)
		}

		if resp.GetStatus() != healthpb.HealthCheckResponse_SERVING {
			log.Fatalf("service not in serving state: ", resp.GetStatus().String())
		}
		log.Printf("RPC HealthChekStatus: %v\n", resp.GetStatus())
	}
	// now make a gRPC call

	r, err := c.SayHello(ctx, &echo.EchoRequest{FirstName: "sal", LastName: "amander"})
	if err != nil {
		log.Fatalf("could not greet: %v", err)
	}
	log.Printf("RPC Response: %v\n", r)

}
