package nsq

import (
	"context"
	"encoding/json"
	"time"

	"github.com/1tsndre/mini-go-project/pkg/logger"
	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/service"
	"github.com/google/uuid"
	"github.com/nsqio/go-nsq"
)

type PaymentResultConsumer struct {
	orderService    service.OrderService
	successConsumer *nsq.Consumer
	failedConsumer  *nsq.Consumer
}

func NewPaymentResultConsumer(orderService service.OrderService) *PaymentResultConsumer {
	return &PaymentResultConsumer{orderService: orderService}
}

func (c *PaymentResultConsumer) Start(lookupdAddr string) error {
	successCfg := nsq.NewConfig()
	successConsumer, err := nsq.NewConsumer(constant.TopicPaymentSuccess, constant.ChannelStoreService, successCfg)
	if err != nil {
		return err
	}
	successConsumer.AddHandler(nsq.HandlerFunc(func(message *nsq.Message) error {
		return c.handlePaymentResult(message, true)
	}))
	if err := successConsumer.ConnectToNSQLookupd(lookupdAddr); err != nil {
		return err
	}
	c.successConsumer = successConsumer

	failedCfg := nsq.NewConfig()
	failedConsumer, err := nsq.NewConsumer(constant.TopicPaymentFailed, constant.ChannelStoreService, failedCfg)
	if err != nil {
		successConsumer.Stop()
		return err
	}
	failedConsumer.AddHandler(nsq.HandlerFunc(func(message *nsq.Message) error {
		return c.handlePaymentResult(message, false)
	}))
	if err := failedConsumer.ConnectToNSQLookupd(lookupdAddr); err != nil {
		successConsumer.Stop()
		return err
	}
	c.failedConsumer = failedConsumer

	ctx := context.Background()
	logger.Info(ctx, "NSQ payment result consumers started")
	return nil
}

func (c *PaymentResultConsumer) Stop() {
	ctx := context.Background()
	if c.successConsumer != nil {
		c.successConsumer.Stop()
	}
	if c.failedConsumer != nil {
		c.failedConsumer.Stop()
	}
	logger.Info(ctx, "NSQ payment result consumers stopped")
}

func (c *PaymentResultConsumer) handlePaymentResult(message *nsq.Message, success bool) error {
	var payload struct {
		OrderID string `json:"order_id"`
	}
	if err := json.Unmarshal(message.Body, &payload); err != nil {
		ctx := context.Background()
		logger.Error(ctx, "failed to unmarshal payment result, skipping", err)
		return nil
	}

	orderID, err := uuid.Parse(payload.OrderID)
	if err != nil {
		ctx := context.Background()
		logger.Error(ctx, "invalid order_id in payment result, skipping", err, map[string]interface{}{
			"order_id": payload.OrderID,
		})
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return c.orderService.ProcessPaymentResult(ctx, orderID, success)
}
