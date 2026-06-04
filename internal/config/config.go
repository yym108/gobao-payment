// Package config 定义 Payment 服务的配置结构。
// 通过 mapstructure tag 支持从环境变量加载（前缀 PAYMENT_）。
package config

// Config 是 Payment 服务当前阶段的完整配置。
// I2 阶段先以 mock 形态订阅订单创建事件，为后续真实支付流程接入预留稳定边界。
type Config struct {
	HTTPAddr               string `mapstructure:"http_addr"`                // HTTP 监听地址，如 ":8080"
	GRPCAddr               string `mapstructure:"grpc_addr"`                // gRPC 监听地址，如 ":9090"
	LogLevel               string `mapstructure:"log_level"`                // 日志级别：debug/info/warn/error
	MySQLDSN               string `mapstructure:"mysql_dsn"`                // Payment MySQL 地址，用于支付单持久化
	NATSURL                string `mapstructure:"nats_url"`                 // NATS 连接地址，用于订阅订单创建事件
	NATSStream             string `mapstructure:"nats_stream"`              // JetStream 流名称，如 "SECKILL"
	OrderCreatedSubject    string `mapstructure:"order_created_subject"`    // 订单创建事件主题，如 "order.created"
	OrderCreatedConsumer   string `mapstructure:"order_created_consumer"`   // 订单创建消费者名称，保证重启后可延续消费位点
	OrderCancelledSubject  string `mapstructure:"order_cancelled_subject"`  // 订单取消事件主题，如 "order.cancelled"
	OrderCancelledConsumer string `mapstructure:"order_cancelled_consumer"` // 订单取消消费者名称，保证重启后可延续消费位点
	PaymentPaidSubject     string `mapstructure:"payment_paid_subject"`     // 支付完成事件主题，供后续真实支付流转使用
	PaymentFailedSubject   string `mapstructure:"payment_failed_subject"`   // 支付失败事件主题，供后续订单状态回写使用
}
