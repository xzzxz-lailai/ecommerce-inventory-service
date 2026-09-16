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

// ListStockInboundsRequest 入库申请列表查询条件。
type ListStockInboundsRequest struct {
	Status   *int8 // 状态筛选，为空时查询全部状态。
	Page     int   // 页码。
	PageSize int   // 每页数量。
}

// StockInboundListItem 入库申请列表项。
type StockInboundListItem struct {
	InboundID  int64     `json:"inbound_id" db:"inbound_id"`   // 入库单ID。
	InboundNo  string    `json:"inbound_no" db:"inbound_no"`   // 入库单号。
	Status     int8      `json:"status" db:"status"`           // 入库单状态。
	Remark     *string   `json:"remark" db:"remark"`           // 入库说明。
	CreatedAt  time.Time `json:"created_at" db:"created_at"`   // 创建时间。
	OperatorID int64     `json:"operator_id" db:"operator_id"` // 创建入库单的运营人员ID。
}

// ListStockInboundsResponse 入库申请分页列表响应。
type ListStockInboundsResponse struct {
	List     []StockInboundListItem `json:"list"`      // 当前页的入库申请。
	Total    int64                  `json:"total"`     // 符合条件的总记录数。
	Page     int                    `json:"page"`      // 当前页码。
	PageSize int                    `json:"page_size"` // 每页数量。
}

// StockInboundDetailResponse 入库申请详情响应。
type StockInboundDetailResponse struct {
	InboundID    int64                            `json:"inbound_id"`    // 入库单ID。
	InboundNo    string                           `json:"inbound_no"`    // 入库单号。
	SupplierName string                           `json:"supplier_name"` // 供应商名称。
	Status       int8                             `json:"status"`        // 入库单状态。
	OperatorID   int64                            `json:"operator_id"`   // 创建入库单的运营人员ID。
	ReviewerID   *int64                           `json:"reviewer_id"`   // 审核人ID。
	SubmittedAt  *time.Time                       `json:"submitted_at"`  // 提交审核时间。
	ReviewedAt   *time.Time                       `json:"reviewed_at"`   // 完成审核时间。
	InboundAt    *time.Time                       `json:"inbound_at"`    // 实际增加库存时间。
	Remark       *string                          `json:"remark"`        // 入库说明。
	ReviewRemark *string                          `json:"review_remark"` // 审核说明或拒绝原因。
	CreatedAt    time.Time                        `json:"created_at"`    // 创建时间。
	UpdatedAt    time.Time                        `json:"updated_at"`    // 最后修改时间。
	Items        []StockInboundDetailItemResponse `json:"items"`         // 该入库单的全部SKU明细。
}
