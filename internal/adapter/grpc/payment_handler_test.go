package grpc

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yym108/gobao-payment/internal/application"
	"github.com/yym108/gobao-payment/internal/domain"
	paymentv1 "github.com/yym108/gobao-proto/gen/go/gobao/payment/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type mockPaymentUseCase struct {
	createFn         func(ctx context.Context, snapshot application.OrderSnapshot) (*domain.Payment, error)
	getByIDFn        func(ctx context.Context, userID, paymentID int64) (*domain.Payment, error)
	getByOrderIDFn   func(ctx context.Context, userID, orderID int64) (*domain.Payment, error)
	mockConfirmPayFn func(ctx context.Context, userID, paymentID int64, result string) (*domain.Payment, error)
}

func (m *mockPaymentUseCase) CreatePaymentFromOrder(ctx context.Context, snapshot application.OrderSnapshot) (*domain.Payment, error) {
	return m.createFn(ctx, snapshot)
}
func (m *mockPaymentUseCase) GetPaymentByID(ctx context.Context, userID, paymentID int64) (*domain.Payment, error) {
	return m.getByIDFn(ctx, userID, paymentID)
}
func (m *mockPaymentUseCase) GetPaymentByOrderID(ctx context.Context, userID, orderID int64) (*domain.Payment, error) {
	return m.getByOrderIDFn(ctx, userID, orderID)
}
func (m *mockPaymentUseCase) MockConfirmPayment(ctx context.Context, userID, paymentID int64, result string) (*domain.Payment, error) {
	return m.mockConfirmPayFn(ctx, userID, paymentID, result)
}

func setupPaymentServer(t *testing.T, uc paymentUseCase) paymentv1.PaymentServiceClient {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	paymentv1.RegisterPaymentServiceServer(grpcServer, NewPaymentHandler(uc))
	go func() { _ = grpcServer.Serve(lis) }()
	t.Cleanup(func() {
		grpcServer.Stop()
		_ = lis.Close()
	})

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return paymentv1.NewPaymentServiceClient(conn)
}

func TestPaymentHandler_GetPayment_Success(t *testing.T) {
	client := setupPaymentServer(t, &mockPaymentUseCase{
		getByIDFn: func(_ context.Context, userID, paymentID int64) (*domain.Payment, error) {
			assert.Equal(t, int64(1001), userID)
			assert.Equal(t, int64(301), paymentID)
			now := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)
			return &domain.Payment{
				ID:        301,
				PaymentNo: "PAY-001",
				OrderID:   101,
				OrderNo:   "ORD-001",
				UserID:    1001,
				Amount:    999900,
				Status:    domain.PaymentStatusPending,
				Channel:   "MOCK",
				CreatedAt: now,
				UpdatedAt: now,
			}, nil
		},
	})

	resp, err := client.GetPayment(context.Background(), &paymentv1.GetPaymentRequest{
		UserId:    1001,
		PaymentId: 301,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(301), resp.GetPayment().GetId())
	assert.Equal(t, int64(101), resp.GetPayment().GetOrderId())
}

func TestPaymentHandler_MockConfirmPayment_InvalidArg(t *testing.T) {
	client := setupPaymentServer(t, &mockPaymentUseCase{
		mockConfirmPayFn: func(_ context.Context, _, _ int64, _ string) (*domain.Payment, error) {
			t.Fatal("unexpected mock confirm call")
			return nil, nil
		},
	})

	_, err := client.MockConfirmPayment(context.Background(), &paymentv1.MockConfirmPaymentRequest{
		UserId:    1001,
		PaymentId: 0,
		Result:    "SUCCESS",
	})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestPaymentToProto_ZeroTimes(t *testing.T) {
	resp := paymentToProto(&domain.Payment{
		ID:        401,
		PaymentNo: "PAY-ZERO",
		OrderID:   201,
		OrderNo:   "ORD-ZERO",
		UserID:    1001,
		Amount:    100,
		Status:    domain.PaymentStatusPending,
		Channel:   "MOCK",
	})

	require.NotNil(t, resp)
	assert.Equal(t, int64(0), resp.GetCreatedAt())
	assert.Equal(t, int64(0), resp.GetUpdatedAt())
	assert.Equal(t, int64(0), resp.GetPaidAt())
}
