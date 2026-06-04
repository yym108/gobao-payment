// Package integration 提供 Payment 服务对外部依赖的接线适配实现。
package integration

import (
	"context"
	"encoding/json"

	"github.com/yym108/gobao-payment/internal/domain"
	"github.com/yym108/gobao-pkg/mq"
)

type paymentResultMessage struct {
	PaymentID string `json:"payment_id"` // 支付单号
	OrderID   int64  `json:"order_id"`   // 订单 ID
	OrderNo   string `json:"order_no"`   // 订单号
	UserID    int64  `json:"user_id"`    // 用户 ID
	Amount    int64  `json:"amount"`     // 金额
	Status    string `json:"status"`     // 支付结果状态
	PaidAt    int64  `json:"paid_at"`    // 支付完成时间
}

// PaymentEventPublisher 基于 NATS JetStream 实现支付结果事件发布器。
type PaymentEventPublisher struct {
	bus              *mq.Bus // 消息总线
	succeededSubject string  // 支付成功主题
	failedSubject    string  // 支付失败主题
}

// NewPaymentEventPublisher 创建支付结果事件发布器。
func NewPaymentEventPublisher(bus *mq.Bus, succeededSubject, failedSubject string) *PaymentEventPublisher {
	return &PaymentEventPublisher{bus: bus, succeededSubject: succeededSubject, failedSubject: failedSubject}
}

// PublishPaymentSucceeded 发布支付成功事件。
func (p *PaymentEventPublisher) PublishPaymentSucceeded(ctx context.Context, payment *domain.Payment) error {
	return p.publish(ctx, p.succeededSubject, payment)
}

// PublishPaymentFailed 发布支付失败事件。
func (p *PaymentEventPublisher) PublishPaymentFailed(ctx context.Context, payment *domain.Payment) error {
	return p.publish(ctx, p.failedSubject, payment)
}

func (p *PaymentEventPublisher) publish(ctx context.Context, subject string, payment *domain.Payment) error {
	if p == nil || p.bus == nil || payment == nil || subject == "" {
		return nil
	}
	var paidAt int64
	if payment.PaidAt != nil {
		paidAt = payment.PaidAt.Unix()
	}
	payload, err := json.Marshal(paymentResultMessage{
		PaymentID: payment.PaymentNo,
		OrderID:   payment.OrderID,
		OrderNo:   payment.OrderNo,
		UserID:    payment.UserID,
		Amount:    payment.Amount,
		Status:    payment.Status,
		PaidAt:    paidAt,
	})
	if err != nil {
		return err
	}
	return p.bus.Publish(ctx, subject, payload)
}
