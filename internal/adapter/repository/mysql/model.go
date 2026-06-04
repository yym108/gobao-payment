// Package mysql 提供 Payment 服务基于 GORM 的 MySQL 仓储实现。
package mysql

import (
	"time"

	"github.com/yym108/gobao-payment/internal/domain"
)

// PaymentModel 是支付主表 GORM 模型。
type PaymentModel struct {
	ID        int64      `gorm:"column:id;primaryKey;autoIncrement"`                      // 支付主键
	PaymentNo string     `gorm:"column:payment_no;type:varchar(64);not null;uniqueIndex"` // 业务支付单号
	OrderID   int64      `gorm:"column:order_id;not null;uniqueIndex"`                    // 关联订单 ID
	OrderNo   string     `gorm:"column:order_no;type:varchar(64);not null;index"`         // 关联订单号
	UserID    int64      `gorm:"column:user_id;not null;index"`                           // 支付用户 ID
	Amount    int64      `gorm:"column:amount;not null"`                                  // 支付金额
	Status    string     `gorm:"column:status;type:varchar(32);not null;index"`           // 支付状态
	Channel   string     `gorm:"column:channel;type:varchar(32);not null;default:'MOCK'"` // 支付渠道
	CreatedAt time.Time  `gorm:"column:created_at"`                                       // 创建时间
	UpdatedAt time.Time  `gorm:"column:updated_at"`                                       // 更新时间
	PaidAt    *time.Time `gorm:"column:paid_at"`                                          // 支付完成时间
}

// TableName 指定支付主表名。
func (PaymentModel) TableName() string { return "payments" }

func paymentToModel(payment *domain.Payment) *PaymentModel {
	if payment == nil {
		return nil
	}
	return &PaymentModel{
		ID:        payment.ID,
		PaymentNo: payment.PaymentNo,
		OrderID:   payment.OrderID,
		OrderNo:   payment.OrderNo,
		UserID:    payment.UserID,
		Amount:    payment.Amount,
		Status:    payment.Status,
		Channel:   payment.Channel,
		CreatedAt: payment.CreatedAt,
		UpdatedAt: payment.UpdatedAt,
		PaidAt:    payment.PaidAt,
	}
}

func paymentToDomain(model *PaymentModel) *domain.Payment {
	if model == nil {
		return nil
	}
	return &domain.Payment{
		ID:        model.ID,
		PaymentNo: model.PaymentNo,
		OrderID:   model.OrderID,
		OrderNo:   model.OrderNo,
		UserID:    model.UserID,
		Amount:    model.Amount,
		Status:    model.Status,
		Channel:   model.Channel,
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
		PaidAt:    model.PaidAt,
	}
}
