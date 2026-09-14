package model

import "time"

const (
	StockLockStatusLocked   int8 = 0 // 已锁定，等待订单支付、取消或超时。
	StockLockStatusDeducted int8 = 1 // 已扣减，订单支付成功。
	StockLockStatusReleased int8 = 2 // 已释放，订单已取消或超时。
)

// StockLock 订单库存锁定记录，对应 stock_locks 表。
type StockLock struct {
	LockID    int64     `db:"lock_id"`    // 锁定记录ID，数据库自增主键。
	OrderNo   string    `db:"order_no"`   // 订单号，来自订单服务。
	SKUID     int64     `db:"sku_id"`     // 被当前订单锁定的SKU ID。
	Quantity  int       `db:"quantity"`   // 当前订单锁定该SKU的数量。
	Status    int8      `db:"status"`     // 锁定状态：0已锁定、1已扣减、2已释放。
	ExpiresAt time.Time `db:"expires_at"` // 锁定过期时间，订单超时后需要释放库存。
	CreatedAt time.Time `db:"created_at"` // 锁定记录创建时间。
	UpdatedAt time.Time `db:"updated_at"` // 锁定状态最后更新时间。
}
