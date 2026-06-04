package main

import (
	"testing"

	"github.com/yym108/gobao-payment/internal/config"
)

// TestSmoke 保留最基础的包级冒烟校验，确保测试入口可正常执行。
func TestSmoke(t *testing.T) {}

// TestMockConfigDefaults 约束 I2 阶段 Payment mock 所需的默认配置值。
// 这样后续 main.go 接入订单创建消费时，主题名与消费者名的约定不会被随意改坏。
func TestMockConfigDefaults(t *testing.T) {
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

	if cfg.HTTPAddr == "" {
		t.Fatal("HTTPAddr must not be empty")
	}
	if cfg.GRPCAddr == "" {
		t.Fatal("GRPCAddr must not be empty")
	}
	if cfg.NATSURL == "" {
		t.Fatal("NATSURL must not be empty")
	}
	if cfg.MySQLDSN == "" {
		t.Fatal("MySQLDSN must not be empty")
	}
	if cfg.NATSStream != "SECKILL" {
		t.Fatalf("unexpected NATSStream: %q", cfg.NATSStream)
	}
	if cfg.OrderCreatedSubject != "order.created" {
		t.Fatalf("unexpected OrderCreatedSubject: %q", cfg.OrderCreatedSubject)
	}
	if cfg.OrderCreatedConsumer != "payment-mock" {
		t.Fatalf("unexpected OrderCreatedConsumer: %q", cfg.OrderCreatedConsumer)
	}
	if cfg.OrderCancelledSubject != "order.cancelled" {
		t.Fatalf("unexpected OrderCancelledSubject: %q", cfg.OrderCancelledSubject)
	}
	if cfg.OrderCancelledConsumer != "payment-order-cancelled" {
		t.Fatalf("unexpected OrderCancelledConsumer: %q", cfg.OrderCancelledConsumer)
	}
	if cfg.PaymentPaidSubject != "payment.paid" {
		t.Fatalf("unexpected PaymentPaidSubject: %q", cfg.PaymentPaidSubject)
	}
	if cfg.PaymentFailedSubject != "payment.failed" {
		t.Fatalf("unexpected PaymentFailedSubject: %q", cfg.PaymentFailedSubject)
	}
}
