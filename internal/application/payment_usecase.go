// Package application 编排 Payment 服务的业务用例。
package application

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/yym108/gobao-payment/internal/domain"
	pkgerrors "github.com/yym108/gobao-pkg/errors"
)

// PaymentEventPublisher 抽象支付结果事件发布能力。
type PaymentEventPublisher interface {
	// PublishPaymentSucceeded 发布支付成功事件。
	PublishPaymentSucceeded(ctx context.Context, payment *domain.Payment) error
	// PublishPaymentFailed 发布支付失败事件。
	PublishPaymentFailed(ctx context.Context, payment *domain.Payment) error
}

// OrderSnapshot 描述 payment 创建支付单所需的最小订单快照。
type OrderSnapshot struct {
	OrderID  int64  // 订单 ID
	OrderNo  string // 订单号
	UserID   int64  // 用户 ID
	Amount   int64  // 应付金额
	Status   string // 订单状态
	Source   string // 事件来源，仅用于日志/跟踪扩展
	Quantity int32  // 商品数量，当前不参与金额计算
}

// PaymentUseCase 负责支付单创建、查询和模拟确认。
type PaymentUseCase struct {
	repo        domain.PaymentRepository // 支付仓储
	eventPub    PaymentEventPublisher    // 支付事件发布器
	timeNowFunc func() time.Time         // 当前时间函数，便于测试替换
}

// NewPaymentUseCase 构造 Payment 应用层实例。
func NewPaymentUseCase(repo domain.PaymentRepository, eventPub PaymentEventPublisher) *PaymentUseCase {
	return &PaymentUseCase{
		repo:        repo,
		eventPub:    eventPub,
		timeNowFunc: time.Now,
	}
}

// CreatePaymentFromOrder 基于订单创建待支付单。
func (uc *PaymentUseCase) CreatePaymentFromOrder(ctx context.Context, snapshot OrderSnapshot) (*domain.Payment, error) {
	if snapshot.OrderID <= 0 {
		return nil, pkgerrors.New(pkgerrors.CodeInvalidArg, "order_id 无效")
	}
	if snapshot.UserID <= 0 {
		return nil, pkgerrors.New(pkgerrors.CodeInvalidArg, "user_id 无效")
	}
	if snapshot.Amount <= 0 {
		return nil, pkgerrors.New(pkgerrors.CodeInvalidArg, "amount 必须大于 0")
	}

	existing, err := uc.repo.FindByOrderID(ctx, snapshot.OrderID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	now := uc.timeNowFunc()
	payment := &domain.Payment{
		PaymentNo: buildPaymentNo(now, snapshot.OrderID),
		OrderID:   snapshot.OrderID,
		OrderNo:   strings.TrimSpace(snapshot.OrderNo),
		UserID:    snapshot.UserID,
		Amount:    snapshot.Amount,
		Status:    domain.PaymentStatusPending,
		Channel:   "MOCK",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := uc.repo.Create(ctx, payment); err != nil {
		return nil, err
	}
	return payment, nil
}

// GetPaymentByID 查询当前用户可见的支付单。
func (uc *PaymentUseCase) GetPaymentByID(ctx context.Context, userID, paymentID int64) (*domain.Payment, error) {
	if userID <= 0 {
		return nil, pkgerrors.New(pkgerrors.CodeInvalidArg, "user_id 无效")
	}
	if paymentID <= 0 {
		return nil, pkgerrors.New(pkgerrors.CodeInvalidArg, "payment_id 无效")
	}
	payment, err := uc.repo.FindByID(ctx, paymentID)
	if err != nil {
		return nil, err
	}
	if payment == nil {
		return nil, pkgerrors.New(pkgerrors.CodeNotFound, "支付单不存在")
	}
	if payment.UserID != userID {
		return nil, pkgerrors.New(pkgerrors.CodeForbidden, "无权访问该支付单")
	}
	return payment, nil
}

// GetPaymentByOrderID 按订单查询当前用户的支付单。
func (uc *PaymentUseCase) GetPaymentByOrderID(ctx context.Context, userID, orderID int64) (*domain.Payment, error) {
	if userID <= 0 {
		return nil, pkgerrors.New(pkgerrors.CodeInvalidArg, "user_id 无效")
	}
	if orderID <= 0 {
		return nil, pkgerrors.New(pkgerrors.CodeInvalidArg, "order_id 无效")
	}
	payment, err := uc.repo.FindByOrderID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if payment == nil {
		return nil, pkgerrors.New(pkgerrors.CodeNotFound, "支付单不存在")
	}
	if payment.UserID != userID {
		return nil, pkgerrors.New(pkgerrors.CodeForbidden, "无权访问该支付单")
	}
	return payment, nil
}

// CancelPaymentByOrderID 在订单关闭后同步收敛待支付支付单。
// 当前最小实现只会把 PENDING 支付单推进为 CANCELLED，已完成或已失败的支付单保持原状。
func (uc *PaymentUseCase) CancelPaymentByOrderID(ctx context.Context, orderID int64) (*domain.Payment, error) {
	if orderID <= 0 {
		return nil, pkgerrors.New(pkgerrors.CodeInvalidArg, "order_id 无效")
	}
	payment, err := uc.repo.FindByOrderID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if payment == nil {
		return nil, pkgerrors.New(pkgerrors.CodeNotFound, "支付单不存在")
	}
	if payment.Status != domain.PaymentStatusPending {
		return payment, nil
	}

	now := uc.timeNowFunc()
	updated, err := uc.repo.UpdateStatus(ctx, payment.ID, domain.PaymentStatusPending, domain.PaymentStatusCancelled, nil, now)
	if err != nil {
		return nil, err
	}
	if !updated {
		latest, findErr := uc.repo.FindByID(ctx, payment.ID)
		if findErr != nil {
			return nil, findErr
		}
		if latest == nil {
			return nil, pkgerrors.New(pkgerrors.CodeNotFound, "支付单不存在")
		}
		return latest, nil
	}

	payment.Status = domain.PaymentStatusCancelled
	payment.UpdatedAt = now
	return payment, nil
}

// MockConfirmPayment 模拟确认支付成功或失败。
func (uc *PaymentUseCase) MockConfirmPayment(ctx context.Context, userID, paymentID int64, result string) (*domain.Payment, error) {
	payment, err := uc.GetPaymentByID(ctx, userID, paymentID)
	if err != nil {
		return nil, err
	}
	if payment.Status != domain.PaymentStatusPending {
		return nil, pkgerrors.New(pkgerrors.CodeFailedPrecondition, "当前支付单状态不可重复确认")
	}

	result = strings.ToUpper(strings.TrimSpace(result))
	var targetStatus string
	switch result {
	case "SUCCESS", "SUCCEEDED", "PAID":
		targetStatus = domain.PaymentStatusSucceeded
	case "FAIL", "FAILED":
		targetStatus = domain.PaymentStatusFailed
	default:
		return nil, pkgerrors.New(pkgerrors.CodeInvalidArg, "result 仅支持 SUCCESS 或 FAILED")
	}

	now := uc.timeNowFunc()
	var paidAt *time.Time
	if targetStatus == domain.PaymentStatusSucceeded {
		paidAt = &now
	}
	updated, err := uc.repo.UpdateStatus(ctx, payment.ID, domain.PaymentStatusPending, targetStatus, paidAt, now)
	if err != nil {
		return nil, err
	}
	if !updated {
		return nil, pkgerrors.New(pkgerrors.CodeFailedPrecondition, "当前支付单状态不可重复确认")
	}
	payment.Status = targetStatus
	payment.UpdatedAt = now
	payment.PaidAt = paidAt

	if uc.eventPub != nil {
		if targetStatus == domain.PaymentStatusSucceeded {
			if err := uc.eventPub.PublishPaymentSucceeded(ctx, payment); err != nil {
				return nil, err
			}
		} else {
			if err := uc.eventPub.PublishPaymentFailed(ctx, payment); err != nil {
				return nil, err
			}
		}
	}
	return payment, nil
}

func buildPaymentNo(now time.Time, orderID int64) string {
	return "PAY-" + now.UTC().Format("20060102150405.000000000") + "-" + strconv.FormatInt(orderID, 10)
}
