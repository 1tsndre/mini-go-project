package nsq

import (
	"encoding/json"
	"log"

	"github.com/1tsndre/mini-go-project/payment-service/internal/service"
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

	log.Println("NSQ order consumer started")
	return nil
}

func (c *OrderConsumer) handleOrderCreated(message *nsq.Message) error {
	var payload struct {
		OrderID     string `json:"order_id"`
		UserID      string `json:"user_id"`
		TotalAmount string `json:"total_amount"`
	}

	if err := json.Unmarshal(message.Body, &payload); err != nil {
		log.Printf("failed to unmarshal order.created, skipping: %v", err)
		return nil
	}

	log.Printf("processing payment for order %s, amount: %s", payload.OrderID, payload.TotalAmount)

	result := c.paymentService.ProcessPayment(payload.OrderID, payload.TotalAmount, "mock")

	response, err := json.Marshal(map[string]string{
		"order_id":   result.OrderID,
		"payment_id": result.PaymentID,
		"message":    result.Message,
	})
	if err != nil {
		log.Printf("failed to marshal payment response for order %s, skipping: %v", result.OrderID, err)
		return nil
	}

	if result.Success {
		return c.producer.Publish(topicPaymentSuccess, response)
	}
	return c.producer.Publish(topicPaymentFailed, response)
}
