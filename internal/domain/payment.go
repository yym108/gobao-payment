// Package domain 定义 Payment 服务的核心领域对象与常量。
package domain

import "time"

// Payment 表示支付聚合根。
// 当前最小实现聚焦模拟支付单，不引入真实第三方渠道字段。
type Payment struct {
	ID        int64      // 支付主键
	PaymentNo string     // 业务支付单号
	OrderID   int64      // 关联订单 ID
	OrderNo   string     // 关联订单号
	UserID    int64      // 支付用户 ID
	Amount    int64      // 支付金额，单位为分
	Status    string     // 支付状态
	Channel   string     // 支付渠道，当前固定为 MOCK
	CreatedAt time.Time  // 创建时间
	UpdatedAt time.Time  // 更新时间
	PaidAt    *time.Time // 支付完成时间，未支付时为空
}

const (
	// PaymentStatusPending 表示支付单已创建，等待模拟确认。
	PaymentStatusPending = "PENDING"
	// PaymentStatusSucceeded 表示支付成功。
	PaymentStatusSucceeded = "SUCCEEDED"
	// PaymentStatusFailed 表示支付失败。
	PaymentStatusFailed = "FAILED"
	// PaymentStatusCancelled 表示支付单因订单关闭而失效，不再允许继续支付。
	PaymentStatusCancelled = "CANCELLED"
)
