# gobao-payment

GoBao 的支付服务仓库，当前承担 mock 支付单创建、确认、失败回写与事件发布链路。

## 作用

- 支付单创建与查询
- mock 支付确认 / 失败
- 订阅订单创建事件并生成支付单
- 发布支付成功 / 失败事件

## 关系

- 依赖 `gobao-proto`、`gobao-pkg`
- 消费订单事件并回写支付状态
- 被 `gobao-gateway` 调用

## 独立使用前准备

单独 clone 本仓后，先执行：

```bash
bash scripts/bootstrap-deps.sh
ln -sfn workspace/gobao-pkg ../gobao-pkg
ln -sfn workspace/gobao-proto ../gobao-proto
```

## 环境变量

可参考仓库根目录 `.env.example`：

- `PAYMENT_MYSQL_DSN`
- `PAYMENT_NATS_URL`
- `PAYMENT_ORDER_CREATED_SUBJECT`
- `PAYMENT_PAYMENT_FAILED_SUBJECT`

## 启动

```bash
go test ./...
go run ./cmd/server
```

如需容器化启动，可直接使用仓库内 `Dockerfile`，或由 `gobao-deploy` / `GoBao` 主仓统一编排。
