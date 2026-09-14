package model

import "time"

const (
	StockInboundStatusPending   int8 = 0 // 待审核。
	StockInboundStatusCompleted int8 = 1 // 审核通过，已增加库存。
	StockInboundStatusRejected  int8 = 2 // 已拒绝或取消。
)

// StockInbound 采购入库明细，对应 stock_inbounds 表。
type StockInbound struct {
	InboundID    int64      `db:"inbound_id"`    // 入库明细ID，数据库自增主键。
	InboundNo    string     `db:"inbound_no"`    // 入库单号，同一次入库申请的多个SKU共用一个编号。
	SupplierName string     `db:"supplier_name"` // 供应商公司名称。
	SKUID        int64      `db:"sku_id"`        // 申请入库的SKU ID，来自商品服务。
	Quantity     int        `db:"quantity"`      // 申请入库数量，必须大于0。
	CostPrice    int64      `db:"cost_price"`    // 单件采购成本，单位为分，不能小于0。
	Status       int8       `db:"status"`        // 入库状态：0待审核、1已入库、2已拒绝或取消。
	OperatorID   int64      `db:"operator_id"`   // 创建入库申请的运营人员ID。
	ReviewerID   *int64     `db:"reviewer_id"`   // 审核该入库申请的Admin ID，未审核时为空。
	InboundAt    *time.Time `db:"inbound_at"`    // 审核通过并实际增加库存的时间，未入库时为空。
	Remark       *string    `db:"remark"`        // 入库说明、拒绝原因或取消原因，可以为空。
	CreatedAt    time.Time  `db:"created_at"`    // 运营创建入库申请的时间。
	UpdatedAt    time.Time  `db:"updated_at"`    // 入库记录最后修改时间。
}

// CreateStockInboundItemRequest 创建入库申请时提交的一条 SKU 明细。
type CreateStockInboundItemRequest struct {
	SKUID     int64 `json:"sku_id" binding:"required,gt=0"`   // SKU ID。
	Quantity  int   `json:"quantity" binding:"required,gt=0"` // 申请入库数量。
	CostPrice int64 `json:"cost_price" binding:"gte=0"`       // 单件采购成本，单位为分。
}

// CreateStockInboundRequest 创建入库申请请求。
type CreateStockInboundRequest struct {
	SupplierName string                          `json:"supplier_name" binding:"required,max=200"` // 供应商公司名称。
	Remark       *string                         `json:"remark" binding:"omitempty,max=255"`       // 入库说明，可以为空。
	Items        []CreateStockInboundItemRequest `json:"items" binding:"required,min=1,dive"`      // 本次申请的 SKU 明细。
}

// CreateStockInboundItemResponse 创建成功后返回的 SKU 明细。
type CreateStockInboundItemResponse struct {
	InboundID int64 `json:"inbound_id"` // 入库明细ID。
	SKUID     int64 `json:"sku_id"`     // SKU ID。
	Quantity  int   `json:"quantity"`   // 申请入库数量。
	CostPrice int64 `json:"cost_price"` // 单件采购成本，单位为分。
}

// CreateStockInboundResponse 创建入库申请响应。
type CreateStockInboundResponse struct {
	InboundNo string                           `json:"inbound_no"` // 入库单号。
	Status    int8                             `json:"status"`     // 状态：0待审核。
	Items     []CreateStockInboundItemResponse `json:"items"`      // 本次创建的 SKU 明细。
}
