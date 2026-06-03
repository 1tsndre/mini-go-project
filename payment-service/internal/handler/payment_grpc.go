package handler

import (
	"context"

	"github.com/1tsndre/mini-go-project/payment-service/internal/constant"
	"github.com/1tsndre/mini-go-project/payment-service/internal/service"
	pb "github.com/1tsndre/mini-go-project/proto/payment"
)

type PaymentGRPCHandler struct {
	pb.UnimplementedPaymentServiceServer
	service *service.PaymentService
}

func NewPaymentGRPCHandler(service *service.PaymentService) *PaymentGRPCHandler {
	return &PaymentGRPCHandler{service: service}
}

func (h *PaymentGRPCHandler) GetPaymentStatus(ctx context.Context, req *pb.GetPaymentStatusRequest) (*pb.GetPaymentStatusResponse, error) {
	payment, ok := h.service.GetPayment(req.OrderId)
	if !ok {
		return &pb.GetPaymentStatusResponse{OrderId: req.OrderId, Status: constant.StatusNotFound}, nil
	}

	return &pb.GetPaymentStatusResponse{
		PaymentId: payment.PaymentID,
		OrderId:   payment.OrderID,
		Status:    payment.Status,
		Amount:    payment.Amount,
		Method:    payment.Method,
	}, nil
}
