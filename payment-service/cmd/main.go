package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/1tsndre/mini-go-project/payment-service/internal/config"
	"github.com/1tsndre/mini-go-project/payment-service/internal/constant"
	nsqconsumer "github.com/1tsndre/mini-go-project/payment-service/internal/nsq"
	"github.com/1tsndre/mini-go-project/payment-service/internal/service"
	"github.com/1tsndre/mini-go-project/pkg/logger"
	"github.com/nsqio/go-nsq"
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

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info(ctx, constant.LogServiceShuttingDown)
	orderConsumer.Stop()
	nsqProducer.Stop()
	logger.Info(ctx, constant.LogServiceStopped)
}
