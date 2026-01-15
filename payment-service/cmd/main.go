package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/1tsndre/mini-go-project/payment-service/internal/config"
	"github.com/1tsndre/mini-go-project/payment-service/internal/handler"
	nsqconsumer "github.com/1tsndre/mini-go-project/payment-service/internal/nsq"
	"github.com/1tsndre/mini-go-project/payment-service/internal/service"
	"github.com/1tsndre/mini-go-project/pkg/logger"
	pb "github.com/1tsndre/mini-go-project/proto/payment"
	"github.com/nsqio/go-nsq"
	"google.golang.org/grpc"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	logger.Init(cfg.App.Env)

	ctx := context.Background()

	paymentSvc := service.NewPaymentService()

	nsqProducer, err := nsq.NewProducer(cfg.NSQ.NsqdAddr, nsq.NewConfig())
	if err != nil {
		logger.Fatal(ctx, "failed to create NSQ producer", err)
	}

	orderConsumer := nsqconsumer.NewOrderConsumer(paymentSvc, nsqProducer)
	if err := orderConsumer.Start(cfg.NSQ.LookupdAddr); err != nil {
		logger.Fatal(ctx, "failed to start NSQ consumer", err)
	}

	grpcServer := grpc.NewServer()
	paymentHandler := handler.NewPaymentGRPCHandler(paymentSvc)
	pb.RegisterPaymentServiceServer(grpcServer, paymentHandler)

	lis, err := net.Listen("tcp", ":"+cfg.Payment.GRPCPort)
	if err != nil {
		logger.Fatal(ctx, "failed to listen on gRPC port", err)
	}

	go func() {
		logger.Info(ctx, "payment gRPC server starting", map[string]interface{}{
			"port": cfg.Payment.GRPCPort,
		})
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatal(ctx, "gRPC server failed", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info(ctx, "shutting down payment service...")
	nsqProducer.Stop()
	grpcServer.GracefulStop()
	logger.Info(ctx, "payment service stopped")
}
