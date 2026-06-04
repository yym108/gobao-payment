package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yym108/gobao-payment/internal/domain"
	pkgerrors "github.com/yym108/gobao-pkg/errors"
)

type mockPaymentRepo struct {
	createFn       func(ctx context.Context, payment *domain.Payment) error
	findByIDFn     func(ctx context.Context, id int64) (*domain.Payment, error)
	findByOrderFn  func(ctx context.Context, orderID int64) (*domain.Payment, error)
	updateStatusFn func(ctx context.Context, id int64, fromStatus, toStatus string, paidAt *time.Time, updatedAt time.Time) (bool, error)
}

func (m *mockPaymentRepo) Create(ctx context.Context, payment *domain.Payment) error {
	return m.createFn(ctx, payment)
}
func (m *mockPaymentRepo) FindByID(ctx context.Context, id int64) (*domain.Payment, error) {
	return m.findByIDFn(ctx, id)
}
func (m *mockPaymentRepo) FindByOrderID(ctx context.Context, orderID int64) (*domain.Payment, error) {
	return m.findByOrderFn(ctx, orderID)
}
func (m *mockPaymentRepo) UpdateStatus(ctx context.Context, id int64, fromStatus, toStatus string, paidAt *time.Time, updatedAt time.Time) (bool, error) {
	return m.updateStatusFn(ctx, id, fromStatus, toStatus, paidAt, updatedAt)
}

type mockPaymentPublisher struct {
	publishSuccessFn func(ctx context.Context, payment *domain.Payment) error
	publishFailedFn  func(ctx context.Context, payment *domain.Payment) error
}

func (m *mockPaymentPublisher) PublishPaymentSucceeded(ctx context.Context, payment *domain.Payment) error {
	if m.publishSuccessFn == nil {
		return nil
	}
	return m.publishSuccessFn(ctx, payment)
}
func (m *mockPaymentPublisher) PublishPaymentFailed(ctx context.Context, payment *domain.Payment) error {
	if m.publishFailedFn == nil {
		return nil
	}
	return m.publishFailedFn(ctx, payment)
}

func TestPaymentUseCase_CreatePaymentFromOrder_Success(t *testing.T) {
	repo := &mockPaymentRepo{
		findByOrderFn: func(_ context.Context, orderID int64) (*domain.Payment, error) {
			if orderID != 101 {
				t.Fatalf("unexpected order id: %d", orderID)
			}
			return nil, nil
		},
		createFn: func(_ context.Context, payment *domain.Payment) error {
			payment.ID = 301
			return nil
		},
	}
	uc := NewPaymentUseCase(repo, &mockPaymentPublisher{})
	uc.timeNowFunc = func() time.Time { return time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC) }

	payment, err := uc.CreatePaymentFromOrder(context.Background(), OrderSnapshot{
		OrderID: 101,
		OrderNo: "ORD-001",
		UserID:  1001,
		Amount:  999900,
		Status:  "CREATED",
		Source:  "order.created",
	})
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	if payment == nil || payment.ID != 301 {
		t.Fatalf("unexpected payment: %+v", payment)
	}
	if payment.Status != domain.PaymentStatusPending {
		t.Fatalf("unexpected status: %s", payment.Status)
	}
	if payment.Channel != "MOCK" {
		t.Fatalf("unexpected channel: %s", payment.Channel)
	}
}

func TestPaymentUseCase_GetPaymentByOrderID_NotOwner(t *testing.T) {
	uc := NewPaymentUseCase(&mockPaymentRepo{
		findByOrderFn: func(_ context.Context, orderID int64) (*domain.Payment, error) {
			return &domain.Payment{ID: 301, OrderID: orderID, UserID: 2002}, nil
		},
	}, &mockPaymentPublisher{})

	_, err := uc.GetPaymentByOrderID(context.Background(), 1001, 101)
	if !pkgerrors.IsCode(err, pkgerrors.CodeForbidden) {
		t.Fatalf("expect forbidden, got %v", err)
	}
}

func TestPaymentUseCase_MockConfirmPayment_Success(t *testing.T) {
	repo := &mockPaymentRepo{
		findByIDFn: func(_ context.Context, id int64) (*domain.Payment, error) {
			return &domain.Payment{ID: id, OrderID: 101, UserID: 1001, Status: domain.PaymentStatusPending}, nil
		},
		updateStatusFn: func(_ context.Context, id int64, fromStatus, toStatus string, paidAt *time.Time, updatedAt time.Time) (bool, error) {
			if fromStatus != domain.PaymentStatusPending || toStatus != domain.PaymentStatusSucceeded || paidAt == nil {
				t.Fatalf("unexpected status transition: %s -> %s paidAt=%v", fromStatus, toStatus, paidAt)
			}
			return true, nil
		},
	}
	pub := &mockPaymentPublisher{
		publishSuccessFn: func(_ context.Context, payment *domain.Payment) error {
			if payment.Status != domain.PaymentStatusSucceeded {
				t.Fatalf("unexpected payment status: %s", payment.Status)
			}
			return nil
		},
	}
	uc := NewPaymentUseCase(repo, pub)
	uc.timeNowFunc = func() time.Time { return time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC) }

	payment, err := uc.MockConfirmPayment(context.Background(), 1001, 301, "success")
	if err != nil {
		t.Fatalf("mock confirm payment: %v", err)
	}
	if payment.Status != domain.PaymentStatusSucceeded || payment.PaidAt == nil {
		t.Fatalf("unexpected payment after confirm: %+v", payment)
	}
}

func TestPaymentUseCase_MockConfirmPayment_InvalidResult(t *testing.T) {
	uc := NewPaymentUseCase(&mockPaymentRepo{
		findByIDFn: func(_ context.Context, id int64) (*domain.Payment, error) {
			return &domain.Payment{ID: id, OrderID: 101, UserID: 1001, Status: domain.PaymentStatusPending}, nil
		},
		updateStatusFn: func(_ context.Context, _ int64, _, _ string, _ *time.Time, _ time.Time) (bool, error) {
			t.Fatal("unexpected update status call")
			return false, nil
		},
	}, &mockPaymentPublisher{})

	_, err := uc.MockConfirmPayment(context.Background(), 1001, 301, "UNKNOWN")
	if !pkgerrors.IsCode(err, pkgerrors.CodeInvalidArg) {
		t.Fatalf("expect invalid arg, got %v", err)
	}
}

func TestPaymentUseCase_CreatePaymentFromOrder_CreateFailed(t *testing.T) {
	uc := NewPaymentUseCase(&mockPaymentRepo{
		findByOrderFn: func(_ context.Context, _ int64) (*domain.Payment, error) { return nil, nil },
		createFn:      func(_ context.Context, _ *domain.Payment) error { return errors.New("create failed") },
	}, &mockPaymentPublisher{})

	_, err := uc.CreatePaymentFromOrder(context.Background(), OrderSnapshot{
		OrderID: 101,
		OrderNo: "ORD-001",
		UserID:  1001,
		Amount:  999900,
	})
	if err == nil {
		t.Fatal("expect create failed")
	}
}

func TestPaymentUseCase_CancelPaymentByOrderID_Success(t *testing.T) {
	now := time.Date(2026, 5, 29, 14, 0, 0, 0, time.UTC)
	uc := NewPaymentUseCase(&mockPaymentRepo{
		findByOrderFn: func(_ context.Context, orderID int64) (*domain.Payment, error) {
			if orderID != 101 {
				t.Fatalf("unexpected order id: %d", orderID)
			}
			return &domain.Payment{
				ID:      301,
				OrderID: orderID,
				OrderNo: "ORD-001",
				UserID:  1001,
				Amount:  999900,
				Status:  domain.PaymentStatusPending,
				Channel: "MOCK",
			}, nil
		},
		updateStatusFn: func(_ context.Context, id int64, fromStatus, toStatus string, paidAt *time.Time, updatedAt time.Time) (bool, error) {
			if id != 301 || fromStatus != domain.PaymentStatusPending || toStatus != domain.PaymentStatusCancelled {
				t.Fatalf("unexpected transition: id=%d from=%s to=%s", id, fromStatus, toStatus)
			}
			if paidAt != nil {
				t.Fatalf("unexpected paidAt: %v", paidAt)
			}
			if !updatedAt.Equal(now) {
				t.Fatalf("unexpected updatedAt: %v", updatedAt)
			}
			return true, nil
		},
	}, &mockPaymentPublisher{})
	uc.timeNowFunc = func() time.Time { return now }

	payment, err := uc.CancelPaymentByOrderID(context.Background(), 101)
	if err != nil {
		t.Fatalf("cancel payment by order id: %v", err)
	}
	if payment == nil || payment.Status != domain.PaymentStatusCancelled {
		t.Fatalf("unexpected payment: %+v", payment)
	}
	if !payment.UpdatedAt.Equal(now) {
		t.Fatalf("unexpected updatedAt: %v", payment.UpdatedAt)
	}
}

func TestPaymentUseCase_CancelPaymentByOrderID_IgnoreNonPending(t *testing.T) {
	uc := NewPaymentUseCase(&mockPaymentRepo{
		findByOrderFn: func(_ context.Context, orderID int64) (*domain.Payment, error) {
			return &domain.Payment{
				ID:      302,
				OrderID: orderID,
				UserID:  1001,
				Status:  domain.PaymentStatusSucceeded,
			}, nil
		},
		updateStatusFn: func(_ context.Context, _ int64, _, _ string, _ *time.Time, _ time.Time) (bool, error) {
			t.Fatal("unexpected update status call")
			return false, nil
		},
	}, &mockPaymentPublisher{})

	payment, err := uc.CancelPaymentByOrderID(context.Background(), 102)
	if err != nil {
		t.Fatalf("cancel non-pending payment: %v", err)
	}
	if payment == nil || payment.Status != domain.PaymentStatusSucceeded {
		t.Fatalf("unexpected payment: %+v", payment)
	}
}
