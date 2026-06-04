// Package grpc 提供 Payment 服务的 gRPC Handler 实现。
package grpc

import (
	"context"

	"github.com/yym108/gobao-payment/internal/application"
	"github.com/yym108/gobao-payment/internal/domain"
	pkgerrors "github.com/yym108/gobao-pkg/errors"
	paymentv1 "github.com/yym108/gobao-proto/gen/go/gobao/payment/v1"
)

type paymentUseCase interface {
	CreatePaymentFromOrder(ctx context.Context, snapshot application.OrderSnapshot) (*domain.Payment, error)
	GetPaymentByID(ctx context.Context, userID, paymentID int64) (*domain.Payment, error)
	GetPaymentByOrderID(ctx context.Context, userID, orderID int64) (*domain.Payment, error)
	MockConfirmPayment(ctx context.Context, userID, paymentID int64, result string) (*domain.Payment, error)
}

// PaymentHandler 实现 proto 生成的 PaymentServiceServer 接口。
type PaymentHandler struct {
	paymentv1.UnimplementedPaymentServiceServer
	paymentUC paymentUseCase
}

// NewPaymentHandler 构造 Payment gRPC Handler。
func NewPaymentHandler(paymentUC paymentUseCase) *PaymentHandler {
	return &PaymentHandler{paymentUC: paymentUC}
}

// GetPayment 查询单笔支付单 RPC。
func (h *PaymentHandler) GetPayment(ctx context.Context, req *paymentv1.GetPaymentRequest) (*paymentv1.GetPaymentResponse, error) {
	if req.GetUserId() <= 0 {
		return nil, pkgerrors.ToGRPCStatus(pkgerrors.New(pkgerrors.CodeInvalidArg, "user_id 必须为正数")).Err()
	}
	if req.GetPaymentId() <= 0 {
		return nil, pkgerrors.ToGRPCStatus(pkgerrors.New(pkgerrors.CodeInvalidArg, "payment_id 必须为正数")).Err()
	}
	payment, err := h.paymentUC.GetPaymentByID(ctx, req.GetUserId(), req.GetPaymentId())
	if err != nil {
		return nil, pkgerrors.ToGRPCStatus(err).Err()
	}
	return &paymentv1.GetPaymentResponse{Payment: paymentToProto(payment)}, nil
}

// GetPaymentByOrder 按订单查询支付单 RPC。
func (h *PaymentHandler) GetPaymentByOrder(ctx context.Context, req *paymentv1.GetPaymentByOrderRequest) (*paymentv1.GetPaymentByOrderResponse, error) {
	if req.GetUserId() <= 0 {
		return nil, pkgerrors.ToGRPCStatus(pkgerrors.New(pkgerrors.CodeInvalidArg, "user_id 必须为正数")).Err()
	}
	if req.GetOrderId() <= 0 {
		return nil, pkgerrors.ToGRPCStatus(pkgerrors.New(pkgerrors.CodeInvalidArg, "order_id 必须为正数")).Err()
	}
	payment, err := h.paymentUC.GetPaymentByOrderID(ctx, req.GetUserId(), req.GetOrderId())
	if err != nil {
		return nil, pkgerrors.ToGRPCStatus(err).Err()
	}
	return &paymentv1.GetPaymentByOrderResponse{Payment: paymentToProto(payment)}, nil
}

// MockConfirmPayment 模拟确认支付结果 RPC。
func (h *PaymentHandler) MockConfirmPayment(ctx context.Context, req *paymentv1.MockConfirmPaymentRequest) (*paymentv1.MockConfirmPaymentResponse, error) {
	if req.GetUserId() <= 0 {
		return nil, pkgerrors.ToGRPCStatus(pkgerrors.New(pkgerrors.CodeInvalidArg, "user_id 必须为正数")).Err()
	}
	if req.GetPaymentId() <= 0 {
		return nil, pkgerrors.ToGRPCStatus(pkgerrors.New(pkgerrors.CodeInvalidArg, "payment_id 必须为正数")).Err()
	}
	payment, err := h.paymentUC.MockConfirmPayment(ctx, req.GetUserId(), req.GetPaymentId(), req.GetResult())
	if err != nil {
		return nil, pkgerrors.ToGRPCStatus(err).Err()
	}
	return &paymentv1.MockConfirmPaymentResponse{Payment: paymentToProto(payment)}, nil
}

func paymentToProto(payment *domain.Payment) *paymentv1.Payment {
	if payment == nil {
		return nil
	}
	var paidAt int64
	if payment.PaidAt != nil {
		paidAt = payment.PaidAt.Unix()
	}
	var createdAt int64
	if !payment.CreatedAt.IsZero() {
		createdAt = payment.CreatedAt.Unix()
	}
	var updatedAt int64
	if !payment.UpdatedAt.IsZero() {
		updatedAt = payment.UpdatedAt.Unix()
	}
	return &paymentv1.Payment{
		Id:        payment.ID,
		PaymentNo: payment.PaymentNo,
		OrderId:   payment.OrderID,
		OrderNo:   payment.OrderNo,
		UserId:    payment.UserID,
		Amount:    payment.Amount,
		Status:    payment.Status,
		Channel:   payment.Channel,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		PaidAt:    paidAt,
	}
}
