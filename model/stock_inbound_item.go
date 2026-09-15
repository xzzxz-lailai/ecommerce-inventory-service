package model

import "time"

// StockInboundItem 入库单SKU明细模型，对应 stock_inbound_items 表。
type StockInboundItem struct {
	ItemID    int64     `db:"item_id"`    // 入库明细ID，数据库自增主键。
	InboundID int64     `db:"inbound_id"` // 所属入库单ID。
	SKUID     int64     `db:"sku_id"`     // 申请入库的SKU ID，来自商品服务。
	Quantity  int       `db:"quantity"`   // 申请入库数量，必须大于0。
	CostPrice int64     `db:"cost_price"` // 单件采购成本，单位为分，不能小于0。
	CreatedAt time.Time `db:"created_at"` // 明细创建时间。
	UpdatedAt time.Time `db:"updated_at"` // 明细最后修改时间。
}

// CreateStockInboundItemRequest 添加入库SKU明细请求。
type CreateStockInboundItemRequest struct {
	SKUID     int64 `json:"sku_id" binding:"required,gt=0"`   // SKU ID。
	Quantity  int   `json:"quantity" binding:"required,gt=0"` // 申请入库数量。
	CostPrice int64 `json:"cost_price" binding:"gte=0"`       // 单件采购成本，单位为分。
}

// CreateStockInboundItemResponse 添加入库SKU明细响应。
type CreateStockInboundItemResponse struct {
	ItemID    int64 `json:"item_id"`    // 入库明细ID。
	InboundID int64 `json:"inbound_id"` // 所属入库单ID。
	SKUID     int64 `json:"sku_id"`     // SKU ID。
	Quantity  int   `json:"quantity"`   // 申请入库数量。
	CostPrice int64 `json:"cost_price"` // 单件采购成本，单位为分。
}
