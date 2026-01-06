package handler

import (
	"context"

	"github.com/1tsndre/mini-go-project/payment-service/internal/service"
	pb "github.com/1tsndre/mini-go-project/proto/payment"
)

type PaymentGRPCHandler struct {
	pb.UnimplementedPaymentServiceServer
	service *service.PaymentService
}

func NewPaymentGRPCHandler(svc *service.PaymentService) *PaymentGRPCHandler {
	return &PaymentGRPCHandler{service: svc}
}

func (h *PaymentGRPCHandler) ProcessPayment(ctx context.Context, req *pb.ProcessPaymentRequest) (*pb.ProcessPaymentResponse, error) {
	result := h.service.ProcessPayment(req.OrderId, req.Amount, req.Method)

	status := "failed"
	if result.Success {
		status = "success"
	}

	return &pb.ProcessPaymentResponse{
		Success:   result.Success,
		PaymentId: result.PaymentID,
		Status:    status,
		Message:   result.Message,
	}, nil
}

func (h *PaymentGRPCHandler) GetPaymentStatus(ctx context.Context, req *pb.GetPaymentStatusRequest) (*pb.GetPaymentStatusResponse, error) {
	return &pb.GetPaymentStatusResponse{
		OrderId: req.OrderId,
		Status:  "mock",
	}, nil
}
