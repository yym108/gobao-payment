package domain

import (
	"context"
	"time"
)

// PaymentRepository 定义支付聚合的仓储抽象。
type PaymentRepository interface {
	// Create 创建支付单。
	Create(ctx context.Context, payment *Payment) error
	// FindByID 按支付单主键查询。
	FindByID(ctx context.Context, id int64) (*Payment, error)
	// FindByOrderID 按订单 ID 查询支付单。
	FindByOrderID(ctx context.Context, orderID int64) (*Payment, error)
	// UpdateStatus 原子更新支付状态。
	UpdateStatus(ctx context.Context, id int64, fromStatus, toStatus string, paidAt *time.Time, updatedAt time.Time) (bool, error)
}
