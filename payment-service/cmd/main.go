package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/1tsndre/mini-go-project/payment-service/internal/config"
	"github.com/1tsndre/mini-go-project/payment-service/internal/constant"
	"github.com/1tsndre/mini-go-project/payment-service/internal/handler"
	nsqconsumer "github.com/1tsndre/mini-go-project/payment-service/internal/nsq"
	"github.com/1tsndre/mini-go-project/payment-service/internal/service"
	"github.com/1tsndre/mini-go-project/pkg/logger"
	pb "github.com/1tsndre/mini-go-project/proto/payment"
	"github.com/nsqio/go-nsq"
	"google.golang.org/grpc"
)

func main() {
	cfg := config.Load()

	logger.Init(cfg.Env)

	ctx := context.Background()

	paymentService := service.NewPaymentService()

	nsqProducer, err := nsq.NewProducer(cfg.NsqdAddr, nsq.NewConfig())
	if err != nil {
		logger.Fatal(ctx, constant.LogNSQProducerFailed, err)
	}

	orderConsumer := nsqconsumer.NewOrderConsumer(paymentService, nsqProducer)
	if err := orderConsumer.Start(cfg.NsqdAddr); err != nil {
		logger.Fatal(ctx, constant.LogNSQConsumerFailed, err)
	}

	grpcServer := grpc.NewServer()
	paymentHandler := handler.NewPaymentGRPCHandler(paymentService)
	pb.RegisterPaymentServiceServer(grpcServer, paymentHandler)

	listener, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		logger.Fatal(ctx, constant.LogGRPCListenFailed, err)
	}

	go func() {
		logger.Info(ctx, constant.LogGRPCServerStarting, logger.Fields{
			"port": cfg.GRPCPort,
		})
		if err := grpcServer.Serve(listener); err != nil {
			logger.Fatal(ctx, constant.LogGRPCServerFailed, err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info(ctx, constant.LogServiceShuttingDown)
	orderConsumer.Stop()
	nsqProducer.Stop()
	grpcServer.GracefulStop()
	logger.Info(ctx, constant.LogServiceStopped)
}
