package model

import "time"

const (
	StockInboundStatusDraft     int8 = 0 // 草稿，可以继续编辑入库单和明细。
	StockInboundStatusPending   int8 = 1 // 待审核，已提交且不能继续编辑。
	StockInboundStatusCompleted int8 = 2 // 已入库，审核通过并已增加库存。
	StockInboundStatusRejected  int8 = 3 // 已拒绝，管理员未通过申请。
	StockInboundStatusCancelled int8 = 4 // 已取消，运营终止入库申请。
)

// StockInbound 入库单主表模型，对应 stock_inbounds 表。
type StockInbound struct {
	InboundID    int64      `db:"inbound_id"`    // 入库单ID，数据库自增主键。
	InboundNo    string     `db:"inbound_no"`    // 入库单号，由后端生成。
	SupplierName string     `db:"supplier_name"` // 本次入库对应的供应商名称。
	Status       int8       `db:"status"`        // 状态：0草稿、1待审核、2已入库、3已拒绝、4已取消。
	OperatorID   int64      `db:"operator_id"`   // 创建入库单的运营人员ID。
	ReviewerID   *int64     `db:"reviewer_id"`   // 审核入库单的Admin ID，未审核时为空。
	SubmittedAt  *time.Time `db:"submitted_at"`  // 运营提交审核的时间，草稿状态时为空。
	ReviewedAt   *time.Time `db:"reviewed_at"`   // Admin完成审核的时间，未审核时为空。
	InboundAt    *time.Time `db:"inbound_at"`    // 审核通过并实际增加库存的时间，未入库时为空。
	Remark       *string    `db:"remark"`        // 运营填写的入库说明，可以为空。
	ReviewRemark *string    `db:"review_remark"` // 审核说明或拒绝原因，可以为空。
	CreatedAt    time.Time  `db:"created_at"`    // 入库单创建时间。
	UpdatedAt    time.Time  `db:"updated_at"`    // 入库单最后修改时间。
}

// CreateStockInboundRequest 创建入库单草稿请求。
type CreateStockInboundRequest struct {
	SupplierName string  `json:"supplier_name" binding:"required,max=200"` // 供应商公司名称。
	Remark       *string `json:"remark" binding:"omitempty,max=255"`       // 入库说明，可以为空。
}

// CreateStockInboundResponse 创建入库单草稿响应。
type CreateStockInboundResponse struct {
	InboundID int64  `json:"inbound_id"` // 入库单ID。
	InboundNo string `json:"inbound_no"` // 入库单号。
	Status    int8   `json:"status"`     // 状态：0草稿。
}

// SubmitStockInboundResponse 提交入库申请响应。
type SubmitStockInboundResponse struct {
	InboundID   int64     `json:"inbound_id"`   // 入库单ID。
	InboundNo   string    `json:"inbound_no"`   // 入库单号。
	Status      int8      `json:"status"`       // 状态：1待审核。
	SubmittedAt time.Time `json:"submitted_at"` // 提交审核时间。
}
