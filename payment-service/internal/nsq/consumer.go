package nsq

import (
	"context"
	"encoding/json"

	"github.com/1tsndre/mini-go-project/payment-service/internal/service"
	"github.com/1tsndre/mini-go-project/pkg/logger"
	"github.com/nsqio/go-nsq"
)

const (
	topicOrderCreated   = "order.created"
	topicPaymentSuccess = "payment.success"
	topicPaymentFailed  = "payment.failed"

	channelPaymentService = "payment-service"
)

type OrderConsumer struct {
	paymentService *service.PaymentService
	producer       *nsq.Producer
}

func NewOrderConsumer(svc *service.PaymentService, producer *nsq.Producer) *OrderConsumer {
	return &OrderConsumer{
		paymentService: svc,
		producer:       producer,
	}
}

func (c *OrderConsumer) Start(lookupdAddr string) error {
	cfg := nsq.NewConfig()
	consumer, err := nsq.NewConsumer(topicOrderCreated, channelPaymentService, cfg)
	if err != nil {
		return err
	}

	consumer.AddHandler(nsq.HandlerFunc(func(message *nsq.Message) error {
		return c.handleOrderCreated(message)
	}))

	if err := consumer.ConnectToNSQLookupd(lookupdAddr); err != nil {
		return err
	}

	logger.Info(context.Background(), "NSQ order consumer started")
	return nil
}

func (c *OrderConsumer) handleOrderCreated(message *nsq.Message) error {
	var payload struct {
		OrderID     string `json:"order_id"`
		UserID      string `json:"user_id"`
		TotalAmount string `json:"total_amount"`
	}

	ctx := context.Background()

	if err := json.Unmarshal(message.Body, &payload); err != nil {
		logger.Error(ctx, "failed to unmarshal order.created, skipping", err)
		return nil
	}

	logger.Info(ctx, "processing payment", map[string]any{"order_id": payload.OrderID, "amount": payload.TotalAmount})

	result := c.paymentService.ProcessPayment(ctx, payload.OrderID, payload.TotalAmount, "mock")

	response, err := json.Marshal(map[string]string{
		"order_id":   result.OrderID,
		"payment_id": result.PaymentID,
		"message":    result.Message,
	})
	if err != nil {
		logger.Error(ctx, "failed to marshal payment response, skipping", err, map[string]any{"order_id": result.OrderID})
		return nil
	}

	if result.Success {
		return c.producer.Publish(topicPaymentSuccess, response)
	}
	return c.producer.Publish(topicPaymentFailed, response)
}
