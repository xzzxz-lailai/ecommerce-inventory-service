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

// SKUStockBalanceResponse 后台查询 SKU 库存余额的响应。
type SKUStockBalanceResponse struct {
	SKUID          int64      `json:"sku_id"`          // 查询的 SKU ID。
	AvailableStock int        `json:"available_stock"` // 当前可购买数量。
	LockedStock    int        `json:"locked_stock"`    // 已被订单锁定的数量。
	UpdatedAt      *time.Time `json:"updated_at"`      // 库存最后变化时间；无库存记录时为空。
}

// ListSKUStocksRequest 后台库存列表的分页参数。
type ListSKUStocksRequest struct {
	Page     int // 页码。
	PageSize int // 每页数量。
}

// SKUStockListItem 后台库存列表中的一条记录。
type SKUStockListItem struct {
	SKUID          int64     `db:"sku_id" json:"sku_id"`                   // SKU ID。
	AvailableStock int       `db:"available_stock" json:"available_stock"` // 可用库存。
	LockedStock    int       `db:"locked_stock" json:"locked_stock"`       // 锁定库存。
	UpdatedAt      time.Time `db:"updated_at" json:"updated_at"`           // 库存最后变化时间。
}

// ListSKUStocksResponse 后台库存分页列表响应。
type ListSKUStocksResponse struct {
	List     []SKUStockListItem `json:"list"`      // 当前页库存记录。
	Total    int64              `json:"total"`     // 符合条件的库存记录总数。
	Page     int                `json:"page"`      // 当前页码。
	PageSize int                `json:"page_size"` // 每页数量。
}
