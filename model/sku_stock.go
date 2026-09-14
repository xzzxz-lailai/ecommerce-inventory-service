package model

import "time"

// SKUStock SKU 库存余额，对应 sku_stocks 表。
type SKUStock struct {
	SKUID          int64     `db:"sku_id"`          // SKU ID，来自商品服务，同时是库存记录主键。
	AvailableStock int       `db:"available_stock"` // 可用库存，表示用户当前可以购买的数量。
	LockedStock    int       `db:"locked_stock"`    // 锁定库存，表示已被未完成订单临时占用的数量。
	CreatedAt      time.Time `db:"created_at"`      // 库存记录创建时间。
	UpdatedAt      time.Time `db:"updated_at"`      // 库存最后变化时间。
}
