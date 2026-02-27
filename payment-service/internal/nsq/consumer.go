package nsq

import (
	"context"
	"encoding/json"

	"github.com/1tsndre/mini-go-project/payment-service/internal/constant"
	"github.com/1tsndre/mini-go-project/payment-service/internal/service"
	"github.com/1tsndre/mini-go-project/pkg/logger"
	"github.com/nsqio/go-nsq"
)

type OrderConsumer struct {
	paymentService *service.PaymentService
	producer       *nsq.Producer
	consumer       *nsq.Consumer
}

func NewOrderConsumer(paymentService *service.PaymentService, producer *nsq.Producer) *OrderConsumer {
	return &OrderConsumer{
		paymentService: paymentService,
		producer:       producer,
	}
}

func (c *OrderConsumer) Start(nsqdAddr string) error {
	cfg := nsq.NewConfig()
	cfg.MaxAttempts = constant.NSQMaxAttempts

	consumer, err := nsq.NewConsumer(constant.TopicOrderCreated, constant.ChannelPaymentService, cfg)
	if err != nil {
		return err
	}

	consumer.AddHandler(nsq.HandlerFunc(c.handleOrderCreated))
	if err := consumer.ConnectToNSQD(nsqdAddr); err != nil {
		return err
	}
	c.consumer = consumer

	logger.Info(context.Background(), constant.LogConsumerStarted)
	return nil
}

func (c *OrderConsumer) Stop() {
	c.consumer.Stop()
	<-c.consumer.StopChan
	logger.Info(context.Background(), constant.LogConsumerStopped)
}

func (c *OrderConsumer) handleOrderCreated(message *nsq.Message) error {
	var (
		ctx   = context.Background()
		topic = constant.TopicPaymentFailed
		order struct {
			OrderID     string `json:"order_id"`
			TotalAmount string `json:"total_amount"`
		}
	)

	if err := json.Unmarshal(message.Body, &order); err != nil {
		logger.Error(ctx, constant.LogOrderUnmarshalFailed, err)
		return nil
	}

	payment := c.paymentService.ProcessPayment(ctx, order.OrderID, order.TotalAmount, constant.MethodMock)
	if payment.Success() {
		topic = constant.TopicPaymentSuccess
	}

	body, err := json.Marshal(map[string]string{
		"order_id":   payment.OrderID,
		"payment_id": payment.PaymentID,
	})
	if err != nil {
		return err
	}
	return c.producer.Publish(topic, body)
}
