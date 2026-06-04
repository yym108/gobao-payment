// Package main 启动 Payment 服务。
// I2 阶段该服务仍为 mock 实现，当前先负责消费订单创建事件并发布支付完成事件，
// 为后续真实支付流程与订单状态流转保留稳定的消息边界。
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os/signal"
	"syscall"
	"time"

	pkgcfg "github.com/yym108/gobao-pkg/config"
	pkgerrors "github.com/yym108/gobao-pkg/errors"
	"github.com/yym108/gobao-pkg/grpcx"
	"github.com/yym108/gobao-pkg/logger"
	"github.com/yym108/gobao-pkg/mq"
	"github.com/yym108/gobao-pkg/server"
	paymentv1 "github.com/yym108/gobao-proto/gen/go/gobao/payment/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	paymentgrpc "github.com/yym108/gobao-payment/internal/adapter/grpc"
	"github.com/yym108/gobao-payment/internal/adapter/integration"
	mysqlrepo "github.com/yym108/gobao-payment/internal/adapter/repository/mysql"
	"github.com/yym108/gobao-payment/internal/application"
	"github.com/yym108/gobao-payment/internal/config"
)

// orderCreatedMessage 是 Order mock 发布给 Payment 的占位事件。
// Payment mock 当前只消费这些字段，不引入真实支付单和三方支付状态。
type orderCreatedMessage struct {
	OrderID    string `json:"order_id"`    // Mock 订单号
	RequestID  string `json:"request_id"`  // 对应的幂等请求 ID
	UserID     int64  `json:"user_id"`     // 下单用户 ID
	ActivityID int64  `json:"activity_id"` // 秒杀活动 ID
	ProductID  int64  `json:"product_id"`  // 商品 ID
	Quantity   int32  `json:"quantity"`    // 下单数量
	Status     string `json:"status"`      // 当前固定为 CREATED，表示订单已创建
	CreatedAt  int64  `json:"created_at"`  // 订单创建事件时间戳
}

type orderAggregateMessage struct {
	ID          int64  `json:"id"`           // 真实订单 ID
	OrderNo     string `json:"order_no"`     // 真实订单号
	UserID      int64  `json:"user_id"`      // 下单用户 ID
	TotalAmount int64  `json:"total_amount"` // 订单总金额
	Status      string `json:"status"`       // 订单状态
}

// orderCancelledMessage 表示 Order 服务发布的订单关闭事件。
// 当前 Payment 只依赖订单 ID 来收敛待支付支付单，其他字段保留给日志和后续扩展使用。
type orderCancelledMessage struct {
	ID      int64  `json:"id"`       // 真实订单 ID
	OrderNo string `json:"order_no"` // 真实订单号
	UserID  int64  `json:"user_id"`  // 下单用户 ID
	Status  string `json:"status"`   // 当前订单状态
}

// main 负责装配 Payment 服务的运行依赖。
// 当前阶段除基础 HTTP/gRPC 健康检查外，还会建立 NATS 订阅来承接订单创建事件。
func main() {
	cfg := config.Config{
		HTTPAddr:               ":8080",
		GRPCAddr:               ":9090",
		LogLevel:               "info",
		MySQLDSN:               "root:root@tcp(mysql-payment:3306)/payment?charset=utf8mb4&parseTime=True&loc=Local",
		NATSURL:                "nats://localhost:4222",
		NATSStream:             "SECKILL",
		OrderCreatedSubject:    "order.created",
		OrderCreatedConsumer:   "payment-mock",
		OrderCancelledSubject:  "order.cancelled",
		OrderCancelledConsumer: "payment-order-cancelled",
		PaymentPaidSubject:     "payment.paid",
		PaymentFailedSubject:   "payment.failed",
	}
	if err := pkgcfg.Load("PAYMENT", "", &cfg); err != nil {
		panic("load payment config: " + err.Error())
	}

	log := logger.New("payment", cfg.LogLevel)
	defer func() { _ = log.Sync() }()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db := mustOpenMySQL(cfg, log)
	defer closeDB(db, log)

	bus := mustBuildBus(cfg, log)
	defer bus.Close()
	paymentRepo := mysqlrepo.NewPaymentRepo(db)
	eventPublisher := integration.NewPaymentEventPublisher(bus, cfg.PaymentPaidSubject, cfg.PaymentFailedSubject)
	paymentUC := application.NewPaymentUseCase(paymentRepo, eventPublisher)
	paymentHandler := paymentgrpc.NewPaymentHandler(paymentUC)
	subscribeOrderCreated(ctx, bus, cfg, log, paymentUC)
	subscribeOrderCancelled(ctx, bus, cfg, log, paymentUC)

	s := server.New("payment", server.Options{
		HTTPAddr: cfg.HTTPAddr,
		GRPCAddr: cfg.GRPCAddr,
		GRPCOpts: []grpc.ServerOption{
			grpc.ChainUnaryInterceptor(grpcx.TraceID(), grpcx.Recover()),
		},
		Register: func(gs *grpc.Server) {
			paymentv1.RegisterPaymentServiceServer(gs, paymentHandler)
		},
	})
	log.Info("starting service",
		zap.String("http_addr", cfg.HTTPAddr),
		zap.String("grpc_addr", cfg.GRPCAddr),
		zap.String("mysql_dsn", cfg.MySQLDSN),
		zap.String("order_created_subject", cfg.OrderCreatedSubject),
		zap.String("order_cancelled_subject", cfg.OrderCancelledSubject),
		zap.String("payment_paid_subject", cfg.PaymentPaidSubject),
		zap.String("payment_failed_subject", cfg.PaymentFailedSubject),
	)
	if err := s.Run(ctx); err != nil {
		log.Fatal("payment service exited unexpectedly", zap.Error(err))
	}
}

// mustBuildBus 建立 Payment mock 所需的 JetStream 总线。
// 这里一次性声明消费主题与后续发布主题，避免未来接真实支付流程时再回头改 Stream 边界。
func mustBuildBus(cfg config.Config, log *zap.Logger) *mq.Bus {
	bus, err := mq.New(mq.Config{
		URL:      cfg.NATSURL,
		Stream:   cfg.NATSStream,
		Subjects: []string{cfg.OrderCreatedSubject, cfg.OrderCancelledSubject, cfg.PaymentPaidSubject, cfg.PaymentFailedSubject},
	})
	if err != nil {
		log.Fatal("failed to initialize payment message bus", zap.Error(err))
	}
	return bus
}

// subscribeOrderCreated 注册订单创建消费者。
// 当前阶段改为：解析秒杀 mock 或真实订单事件 → 自动创建待支付单。
func subscribeOrderCreated(ctx context.Context, bus *mq.Bus, cfg config.Config, log *zap.Logger, paymentUC *application.PaymentUseCase) {
	err := bus.Subscribe(ctx, cfg.OrderCreatedConsumer, cfg.OrderCreatedSubject, func(ctx context.Context, payload []byte) error {
		var msg orderCreatedMessage
		if err := json.Unmarshal(payload, &msg); err == nil && msg.OrderID != "" {
			log.Info("received mock order created message",
				zap.String("order_id", msg.OrderID),
				zap.String("request_id", msg.RequestID),
				zap.Int64("user_id", msg.UserID),
				zap.Int64("activity_id", msg.ActivityID),
				zap.Int64("product_id", msg.ProductID),
				zap.Int32("quantity", msg.Quantity),
				zap.String("status", msg.Status),
			)
			_, err := paymentUC.CreatePaymentFromOrder(ctx, application.OrderSnapshot{
				OrderID:  hashMockOrderID(msg.OrderID),
				OrderNo:  msg.OrderID,
				UserID:   msg.UserID,
				Amount:   int64(msg.Quantity) * 100,
				Status:   msg.Status,
				Source:   "seckill-mock",
				Quantity: msg.Quantity,
			})
			return err
		}

		var orderMsg orderAggregateMessage
		if err := json.Unmarshal(payload, &orderMsg); err != nil {
			return err
		}
		log.Info("received order aggregate message",
			zap.Int64("order_id", orderMsg.ID),
			zap.String("order_no", orderMsg.OrderNo),
			zap.Int64("user_id", orderMsg.UserID),
			zap.Int64("amount", orderMsg.TotalAmount),
			zap.String("status", orderMsg.Status),
		)
		_, err := paymentUC.CreatePaymentFromOrder(ctx, application.OrderSnapshot{
			OrderID: orderMsg.ID,
			OrderNo: orderMsg.OrderNo,
			UserID:  orderMsg.UserID,
			Amount:  orderMsg.TotalAmount,
			Status:  orderMsg.Status,
			Source:  "order-service",
		})
		return err
	})
	if err != nil {
		log.Fatal("failed to subscribe order created events", zap.Error(err))
	}
}

// subscribeOrderCancelled 注册订单关闭消费者。
// 当前最小实现只负责把对应的待支付支付单推进为 CANCELLED，避免前端继续看到可支付状态。
func subscribeOrderCancelled(ctx context.Context, bus *mq.Bus, cfg config.Config, log *zap.Logger, paymentUC *application.PaymentUseCase) {
	err := bus.Subscribe(ctx, cfg.OrderCancelledConsumer, cfg.OrderCancelledSubject, func(ctx context.Context, payload []byte) error {
		var msg orderCancelledMessage
		if err := json.Unmarshal(payload, &msg); err != nil {
			return err
		}
		log.Info("received order cancelled message",
			zap.Int64("order_id", msg.ID),
			zap.String("order_no", msg.OrderNo),
			zap.Int64("user_id", msg.UserID),
			zap.String("status", msg.Status),
		)
		_, err := paymentUC.CancelPaymentByOrderID(ctx, msg.ID)
		if err != nil && pkgerrors.IsCode(err, pkgerrors.CodeNotFound) {
			log.Info("payment not found for cancelled order, skip",
				zap.Int64("order_id", msg.ID),
			)
			return nil
		}
		return err
	})
	if err != nil {
		log.Fatal("failed to subscribe order cancelled events", zap.Error(err))
	}
}

// hashMockOrderID 为秒杀 mock 订单号派生一个稳定 int64，便于沿用真实支付表主键关联规则。
func hashMockOrderID(orderID string) int64 {
	var result int64
	for _, ch := range orderID {
		result = result*131 + int64(ch)
	}
	if result < 0 {
		return -result
	}
	return result
}

func mustOpenMySQL(cfg config.Config, log *zap.Logger) *gorm.DB {
	var (
		db  *gorm.DB
		err error
	)
	for i := 1; i <= 30; i++ {
		db, err = gorm.Open(mysql.Open(cfg.MySQLDSN), &gorm.Config{})
		if err == nil {
			sqlDB, pingErr := db.DB()
			if pingErr == nil {
				pingErr = sqlDB.Ping()
			}
			if pingErr == nil {
				break
			}
			err = pingErr
		}
		log.Warn("payment mysql not ready, retrying "+fmt.Sprintf("%d/30", i), zap.Error(err))
		time.Sleep(time.Second)
	}
	if err != nil {
		log.Fatal("failed to connect payment mysql", zap.Error(err))
	}
	if err := db.AutoMigrate(&mysqlrepo.PaymentModel{}); err != nil {
		log.Fatal("failed to migrate payment tables", zap.Error(err))
	}
	return db
}

func closeDB(db *gorm.DB, log *zap.Logger) {
	sqlDB, err := db.DB()
	if err != nil {
		log.Warn("failed to obtain payment sql db", zap.Error(err))
		return
	}
	if err := sqlDB.Close(); err != nil {
		log.Warn("failed to close payment sql db", zap.Error(err))
	}
}
