package mysql

import (
	"context"
	"time"

	"github.com/yym108/gobao-payment/internal/domain"
	"gorm.io/gorm"
)

// PaymentRepo 基于 GORM 实现 PaymentRepository。
type PaymentRepo struct {
	db *gorm.DB // GORM 数据库连接
}

// NewPaymentRepo 创建 PaymentRepo。
func NewPaymentRepo(db *gorm.DB) *PaymentRepo {
	return &PaymentRepo{db: db}
}

// Create 创建支付单。
func (r *PaymentRepo) Create(ctx context.Context, payment *domain.Payment) error {
	model := paymentToModel(payment)
	if err := r.db.WithContext(ctx).Create(model).Error; err != nil {
		return err
	}
	payment.ID = model.ID
	return nil
}

// FindByID 按支付单主键查询。
func (r *PaymentRepo) FindByID(ctx context.Context, id int64) (*domain.Payment, error) {
	var model PaymentModel
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&model).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return paymentToDomain(&model), nil
}

// FindByOrderID 按订单 ID 查询支付单。
func (r *PaymentRepo) FindByOrderID(ctx context.Context, orderID int64) (*domain.Payment, error) {
	var model PaymentModel
	if err := r.db.WithContext(ctx).Where("order_id = ?", orderID).First(&model).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return paymentToDomain(&model), nil
}

// UpdateStatus 在旧状态匹配时原子更新支付状态。
func (r *PaymentRepo) UpdateStatus(ctx context.Context, id int64, fromStatus, toStatus string, paidAt *time.Time, updatedAt time.Time) (bool, error) {
	update := map[string]any{
		"status":     toStatus,
		"updated_at": updatedAt,
	}
	if paidAt != nil {
		update["paid_at"] = *paidAt
	}
	tx := r.db.WithContext(ctx).
		Model(&PaymentModel{}).
		Where("id = ? AND status = ?", id, fromStatus).
		Updates(update)
	if tx.Error != nil {
		return false, tx.Error
	}
	return tx.RowsAffected > 0, nil
}
