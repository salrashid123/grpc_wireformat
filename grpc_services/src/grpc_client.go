package main

import (
	"flag"
	"time"

	echo "github.com/salrashid123/grpc_dynamic_pb/example/src/echo"

	log "github.com/golang/glog"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

const ()

var (
	conn *grpc.ClientConn
)

func main() {

	address := flag.String("host", "localhost:50051", "host:port of gRPC server")
	skipHealthCheck := flag.Bool("skipHealthCheck", false, "Skip Initial Healthcheck")

	flag.Parse()

	var err error
	var conn *grpc.ClientConn

	conn, err = grpc.Dial(*address, grpc.WithInsecure())

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
		log.Infof("RPC HealthChekStatus: %v\n", resp.GetStatus())
	}
	// now make a gRPC call

	r, err := c.SayHello(ctx, &echo.EchoRequest{FirstName: "sal", LastName: "amander"})
	if err != nil {
		log.Fatalf("could not greet: %v", err)
	}
	log.Infof("RPC Response: %v\n", r)

}
