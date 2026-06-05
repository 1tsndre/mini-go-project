package payment

import (
	"context"

	pb "github.com/1tsndre/mini-go-project/proto/payment"
	"github.com/1tsndre/mini-go-project/store-service/internal/model"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	conn   *grpc.ClientConn
	client pb.PaymentServiceClient
}

func NewClient(addr string) (*Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn, client: pb.NewPaymentServiceClient(conn)}, nil
}

func (c *Client) GetStatus(ctx context.Context, orderID string) (*model.PaymentStatusResponse, error) {
	resp, err := c.client.GetPaymentStatus(ctx, &pb.GetPaymentStatusRequest{OrderId: orderID})
	if err != nil {
		return nil, err
	}
	return &model.PaymentStatusResponse{
		OrderID:   resp.OrderId,
		PaymentID: resp.PaymentId,
		Status:    resp.Status,
		Amount:    resp.Amount,
		Method:    resp.Method,
	}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}
