package model

import "time"

const (
	StockBusinessTypeInbound      int8 = 1 // 采购审核入库。
	StockBusinessTypeOrderLock    int8 = 2 // 创建订单并锁定库存。
	StockBusinessTypeOrderDeduct  int8 = 3 // 订单支付成功后扣减锁定库存。
	StockBusinessTypeOrderRelease int8 = 4 // 订单取消或超时后释放库存。
)

// StockLog 库存流水，对应 stock_logs 表。
type StockLog struct {
	LogID           int64     `db:"log_id"`           // 库存流水ID，数据库自增主键。
	SKUID           int64     `db:"sku_id"`           // 发生库存变化的SKU ID。
	BusinessType    int8      `db:"business_type"`    // 业务类型：1入库、2订单锁定、3订单扣减、4订单释放。
	BusinessNo      string    `db:"business_no"`      // 业务单号：入库时为inbound_no，订单业务时为order_no。
	AvailableChange int       `db:"available_change"` // 可用库存变化量，正数表示增加，负数表示减少。
	LockedChange    int       `db:"locked_change"`    // 锁定库存变化量，正数表示增加，负数表示减少。
	OperatorID      *int64    `db:"operator_id"`      // 操作人ID；系统自动执行时为空。
	Remark          *string   `db:"remark"`           // 库存变化的补充说明，可以为空。
	CreatedAt       time.Time `db:"created_at"`       // 库存变化发生时间。
}
