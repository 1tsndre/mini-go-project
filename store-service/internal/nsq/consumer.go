package nsq

import (
	"context"
	"encoding/json"

	"github.com/1tsndre/mini-go-project/pkg/logger"
	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/service"
	"github.com/google/uuid"
	"github.com/nsqio/go-nsq"
)

type PaymentResultConsumer struct {
	orderService service.OrderService
	consumers    []*nsq.Consumer
}

func NewPaymentResultConsumer(orderService service.OrderService) *PaymentResultConsumer {
	return &PaymentResultConsumer{orderService: orderService}
}

func (c *PaymentResultConsumer) Start(nsqdAddr string) error {
	cfg := nsq.NewConfig()
	cfg.MaxAttempts = constant.NSQMaxAttempts

	results := map[string]bool{constant.TopicPaymentSuccess: true, constant.TopicPaymentFailed: false}
	for topic, success := range results {
		consumer, err := nsq.NewConsumer(topic, constant.ChannelStoreService, cfg)
		if err != nil {
			return err
		}

		consumer.AddHandler(nsq.HandlerFunc(func(message *nsq.Message) error {
			return c.handlePaymentResult(message, success)
		}))
		if err := consumer.ConnectToNSQD(nsqdAddr); err != nil {
			return err
		}
		c.consumers = append(c.consumers, consumer)
	}

	logger.Info(context.Background(), constant.LogConsumersStarted)
	return nil
}

func (c *PaymentResultConsumer) Stop() {
	for _, consumer := range c.consumers {
		consumer.Stop()
	}

	for _, consumer := range c.consumers {
		<-consumer.StopChan
	}
	logger.Info(context.Background(), constant.LogConsumersStopped)
}

func (c *PaymentResultConsumer) handlePaymentResult(message *nsq.Message, success bool) error {
	var payload struct {
		OrderID string `json:"order_id"`
	}

	if err := json.Unmarshal(message.Body, &payload); err != nil {
		logger.Error(context.Background(), constant.LogPaymentResultUnmarshalFailed, err)
		return nil
	}

	orderID, err := uuid.Parse(payload.OrderID)
	if err != nil {
		logger.Error(context.Background(), constant.LogPaymentResultInvalidOrderID, err, logger.Fields{
			"order_id": payload.OrderID,
		})
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), constant.NSQHandleTimeout)
	defer cancel()

	return c.orderService.ProcessPaymentResult(ctx, orderID, success)
}
