package authz

import (
	"context"
	"net"
	"testing"
	"time"

	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	grpcserver "github.com/huynhanx03/go-common/pkg/common/grpc/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func TestGRPCServerServesAndShutsDown(t *testing.T) {
	server, err := grpcserver.New(grpcserver.Config{Name: "envoy-ext-authz-grpc-server", Address: "127.0.0.1:0"}, func(server *grpc.Server) {
		authv3.RegisterAuthorizationServer(server, NewCheckServer(nil, nil, nil, "access"))
	})
	if err != nil {
		t.Fatalf("grpcserver.New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Run(ctx) }()
	deadline := time.Now().Add(time.Second)
	for server.Address() == "" && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	connection, err := net.DialTimeout("tcp", server.Address(), time.Second)
	if err != nil {
		t.Fatalf("gRPC listener unavailable: %v", err)
	}
	_ = connection.Close()
	client, err := grpc.NewClient(server.Address(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	checkCtx, checkCancel := context.WithTimeout(context.Background(), time.Second)
	defer checkCancel()
	health, err := healthpb.NewHealthClient(client).Check(checkCtx, &healthpb.HealthCheckRequest{})
	if err != nil || health.GetStatus() != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("gRPC health = %v, %v", health, err)
	}
	cancel()
	if err := server.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestLoadServerMTLSRejectsPartialConfiguration(t *testing.T) {
	if _, err := LoadServerMTLS("certificate", "", "ca"); err == nil {
		t.Fatal("LoadServerMTLS partial configuration error = nil")
	}
	config, err := LoadServerMTLS("", "", "")
	if err != nil || config != nil {
		t.Fatalf("LoadServerMTLS local configuration = %#v, %v; want nil, nil", config, err)
	}
}
