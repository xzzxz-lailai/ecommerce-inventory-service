package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
	"unicode/utf8"

	"inventory-service/global"
	"inventory-service/httpclient"
	"inventory-service/model"
	"inventory-service/repo"
)

var (
	ErrInvalidStockInbound        = errors.New("入库申请参数错误")
	ErrStockInboundNotFound       = errors.New("入库单不存在")
	ErrStockInboundForbidden      = errors.New("无权操作该入库单")
	ErrStockInboundNotDraft       = errors.New("只有草稿状态的入库单可以操作明细")
	ErrStockInboundNotCancellable = errors.New("只有草稿或待审核的入库单可以取消")
	ErrStockInboundNotPending     = errors.New("只有待审核的入库单可以审核")
	ErrStockInboundItemsEmpty     = errors.New("入库单至少需要一条SKU明细")
	ErrStockInboundItemNotFound   = errors.New("入库单SKU明细不存在")
	ErrStockInboundItemDuplicate  = errors.New("该入库单中已经存在此SKU")
	ErrProductSKUNotFound         = errors.New("SKU不存在")
	ErrProductNotFound            = errors.New("商品不存在")
	ErrProductServiceUnavailable  = errors.New("商品服务暂时不可用")
)

const (
	inboundNoGenerateAttempts   = 5
	defaultStockInboundPage     = 1
	defaultStockInboundPageSize = 10
	maxStockInboundPageSize     = 100
)

// CreateStockInbound 创建一张采购入库单草稿。
func CreateStockInbound(ctx context.Context, req *model.CreateStockInboundRequest, operatorID int64) (*model.CreateStockInboundResponse, error) {
	// 1.校验供应商名称和运营人员ID。
	supplierName := strings.TrimSpace(req.SupplierName)
	if supplierName == "" {
		return nil, fmt.Errorf("%w：供应商名称不能为空", ErrInvalidStockInbound)
	}
	if operatorID <= 0 {
		return nil, fmt.Errorf("%w：运营人员ID不能为空", ErrInvalidStockInbound)
	}

	// 2.生成唯一的入库单号。
	inboundNo, err := generateUniqueInboundNo(ctx)
	if err != nil {
		return nil, err
	}

	// 3.组装草稿并写入入库单主表。
	inbound := &model.StockInbound{
		InboundNo:    inboundNo,
		SupplierName: supplierName,
		Status:       model.StockInboundStatusDraft,
		OperatorID:   operatorID,
		Remark:       req.Remark,
	}
	if err := repo.CreateStockInbound(ctx, inbound); err != nil {
		return nil, err
	}

	// 4.返回新建草稿的主表信息。
	return &model.CreateStockInboundResponse{
		InboundID: inbound.InboundID,
		InboundNo: inbound.InboundNo,
		Status:    inbound.Status,
	}, nil
}

// CreateStockInboundItem 向一张草稿入库单添加SKU明细
func CreateStockInboundItem(ctx context.Context, inboundID int64, req *model.CreateStockInboundItemRequest, operatorID int64, authorization string) (*model.CreateStockInboundItemResponse, error) {
	// 1.校验入库单ID、运营人员ID和明细参数
	if inboundID <= 0 {
		return nil, fmt.Errorf("%w：入库单ID必须大于0", ErrInvalidStockInbound)
	}
	if operatorID <= 0 {
		return nil, fmt.Errorf("%w：运营人员ID不能为空", ErrInvalidStockInbound)
	}
	if req.SKUID <= 0 {
		return nil, fmt.Errorf("%w：SKU ID必须大于0", ErrInvalidStockInbound)
	}
	if req.Quantity <= 0 {
		return nil, fmt.Errorf("%w：入库数量必须大于0", ErrInvalidStockInbound)
	}
	if req.CostPrice < 0 {
		return nil, fmt.Errorf("%w：采购成本不能小于0", ErrInvalidStockInbound)
	}

	// 2.确认入库单存在、属于当前运营人员并且处于草稿状态
	inbound, err := repo.GetStockInboundByID(ctx, inboundID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrStockInboundNotFound
		}
		return nil, err
	}
	if inbound.OperatorID != operatorID {
		return nil, ErrStockInboundForbidden
	} // 创建入库申请单的运营人员，与向该申请单添加 SKU 明细的运营人员，必须是同一个人
	if inbound.Status != model.StockInboundStatusDraft {
		return nil, ErrStockInboundNotDraft
	}

	// 3.调用商品服务确认SKU存在
	sku, err := httpclient.GetProductSKU(ctx, req.SKUID, authorization)
	if err != nil {
		if errors.Is(err, httpclient.ErrProductSKUNotFound) {
			return nil, fmt.Errorf("%w：%d", ErrProductSKUNotFound, req.SKUID)
		}
		return nil, ErrProductServiceUnavailable
	}
	// 商品服务返回的 SKUID和前端填入的skuid是否一致,商品服务接防止口返回错误数据
	if sku.SKUID != req.SKUID {
		return nil, ErrProductServiceUnavailable
	}
	// 4.写入SKU明细，数据库唯一索引负责最终防重复
	item := &model.StockInboundItem{
		InboundID: inboundID,
		SKUID:     req.SKUID,
		Quantity:  req.Quantity,
		CostPrice: req.CostPrice,
	}
	if err := repo.CreateStockInboundItem(ctx, item, operatorID); err != nil {
		switch {
		case errors.Is(err, repo.ErrDuplicateStockInboundItem):
			return nil, ErrStockInboundItemDuplicate
		case errors.Is(err, repo.ErrStockInboundNotEditable):
			return nil, ErrStockInboundNotDraft
		default:
			return nil, err
		}
	}

	// 5.返回新增的明细信息。
	return &model.CreateStockInboundItemResponse{
		ItemID:    item.ItemID,
		InboundID: item.InboundID,
		SKUID:     item.SKUID,
		Quantity:  item.Quantity,
		CostPrice: item.CostPrice,
	}, nil
}

// UpdateStockInboundItem 修改当前运营人员草稿入库单中的一条SKU明细。
func UpdateStockInboundItem(ctx context.Context, inboundID int64, itemID int64, req *model.UpdateStockInboundItemRequest, operatorID int64) (*model.UpdateStockInboundItemResponse, bool, error) {
	if inboundID <= 0 || itemID <= 0 || operatorID <= 0 || req == nil || req.CostPrice == nil {
		return nil, false, fmt.Errorf("%w：入库单、明细或请求参数错误", ErrInvalidStockInbound)
	}
	if req.Quantity <= 0 || *req.CostPrice < 0 {
		return nil, false, fmt.Errorf("%w：入库数量必须大于0，采购成本不能小于0", ErrInvalidStockInbound)
	}

	tx, err := global.DB.BeginTxx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()

	// 与提交审核使用同一张主表行锁，确保修改时仍处于草稿状态。
	inbound, err := repo.GetStockInboundByIDForUpdate(ctx, tx, inboundID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, ErrStockInboundNotFound
		}
		return nil, false, err
	}
	if inbound.OperatorID != operatorID { // 创建这张入库单草稿的运营人员 ID，operatorID 是当前登录的运营人员 ID,必须一致
		return nil, false, ErrStockInboundForbidden
	}
	if inbound.Status != model.StockInboundStatusDraft {
		return nil, false, ErrStockInboundNotDraft
	}

	item, err := repo.GetStockInboundItemByIDForUpdate(ctx, tx, inboundID, itemID) // 在当前事务中明细表锁住这张入库单的这一条明细
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, ErrStockInboundItemNotFound
		}
		return nil, false, err
	}

	resp := &model.UpdateStockInboundItemResponse{
		ItemID:    item.ItemID,
		InboundID: item.InboundID,
		SKUID:     item.SKUID,
		Quantity:  req.Quantity,
		CostPrice: *req.CostPrice,
	}

	// 新旧值相同，直接返回成功，并告知前端本次没有修改。
	if item.Quantity == req.Quantity && item.CostPrice == *req.CostPrice {
		if err := tx.Commit(); err != nil {
			return nil, false, err
		}
		return resp, false, nil
	}

	if err := repo.UpdateStockInboundItem(ctx, tx, inboundID, itemID, req.Quantity, *req.CostPrice); err != nil {
		return nil, false, err
	}
	if err := tx.Commit(); err != nil {
		return nil, false, err
	}
	return resp, true, nil
}

// DeleteStockInboundItem 删除当前运营人员草稿入库单中的一条SKU明细。
func DeleteStockInboundItem(ctx context.Context, inboundID int64, itemID int64, operatorID int64) (*model.DeleteStockInboundItemResponse, error) {
	if inboundID <= 0 || itemID <= 0 || operatorID <= 0 {
		return nil, fmt.Errorf("%w：入库单ID、明细ID或运营人员ID错误", ErrInvalidStockInbound)
	}

	tx, err := global.DB.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// 锁定主表行，与修改明细和提交审核的状态检查保持一致。
	inbound, err := repo.GetStockInboundByIDForUpdate(ctx, tx, inboundID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrStockInboundNotFound
		}
		return nil, err
	}
	if inbound.OperatorID != operatorID { //创建入库草稿的人和当前要执行的删除的人,必须一致
		return nil, ErrStockInboundForbidden
	}
	if inbound.Status != model.StockInboundStatusDraft {
		return nil, ErrStockInboundNotDraft
	}

	item, err := repo.GetStockInboundItemByIDForUpdate(ctx, tx, inboundID, itemID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrStockInboundItemNotFound
		}
		return nil, err
	}
	if err := repo.DeleteStockInboundItem(ctx, tx, inboundID, itemID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &model.DeleteStockInboundItemResponse{
		ItemID:    item.ItemID,
		InboundID: item.InboundID,
		SKUID:     item.SKUID,
	}, nil
}

// SubmitStockInbound 提交草稿入库单，等待管理员审核。
func SubmitStockInbound(ctx context.Context, inboundID int64, operatorID int64) (*model.SubmitStockInboundResponse, error) {
	// 1.校验入库单ID和运营人员ID。
	if inboundID <= 0 {
		return nil, fmt.Errorf("%w：入库单ID必须大于0", ErrInvalidStockInbound)
	}
	if operatorID <= 0 {
		return nil, fmt.Errorf("%w：运营人员ID不能为空", ErrInvalidStockInbound)
	}

	// 2.开启事务并锁定入库单，防止提交期间继续修改明细。
	tx, err := global.DB.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	inbound, err := repo.GetStockInboundByIDForUpdate(ctx, tx, inboundID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrStockInboundNotFound
		}
		return nil, err
	}
	// 创建入库单和提交审核必须是同一个人,防止不同运营人员修改或提交别人的草稿
	if inbound.OperatorID != operatorID {
		return nil, ErrStockInboundForbidden
	}
	if inbound.Status != model.StockInboundStatusDraft {
		return nil, ErrStockInboundNotDraft
	}

	// 3.入库单至少需要一条SKU明细才能提交。
	itemCount, err := repo.CountStockInboundItems(ctx, tx, inboundID)
	if err != nil {
		return nil, err
	}
	if itemCount == 0 {
		return nil, ErrStockInboundItemsEmpty
	}

	// 4.将草稿更新为待审核并记录提交时间。
	submittedAt := time.Now()
	if err := repo.SubmitStockInbound(ctx, tx, inboundID, submittedAt); err != nil {
		if errors.Is(err, repo.ErrStockInboundNotEditable) {
			return nil, ErrStockInboundNotDraft
		}
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// 5.返回提交结果。
	return &model.SubmitStockInboundResponse{
		InboundID:   inbound.InboundID,
		InboundNo:   inbound.InboundNo,
		Status:      model.StockInboundStatusPending,
		SubmittedAt: submittedAt,
	}, nil
}

// CancelStockInbound 取消当前运营人员创建的草稿或待审核入库单。
func CancelStockInbound(ctx context.Context, inboundID int64, operatorID int64) (*model.CancelStockInboundResponse, error) {
	if inboundID <= 0 || operatorID <= 0 {
		return nil, fmt.Errorf("%w：入库单ID或运营人员ID错误", ErrInvalidStockInbound)
	}

	tx, err := global.DB.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// 与管理员审核使用同一张主表行锁，避免取消和审核同时成功。
	inbound, err := repo.GetStockInboundByIDForUpdate(ctx, tx, inboundID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrStockInboundNotFound
		}
		return nil, err
	}
	if inbound.OperatorID != operatorID { // 入库申请草稿单的人和要执行取消入库申请草稿单的人,必须一致
		return nil, ErrStockInboundForbidden
	}
	// 如果当前状态不是草稿和待审核,就返回
	if inbound.Status != model.StockInboundStatusDraft && inbound.Status != model.StockInboundStatusPending {
		return nil, ErrStockInboundNotCancellable
	}

	if err := repo.CancelStockInbound(ctx, tx, inboundID, operatorID); err != nil {
		if errors.Is(err, repo.ErrStockInboundNotEditable) {
			return nil, ErrStockInboundNotCancellable
		}
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &model.CancelStockInboundResponse{
		InboundID: inbound.InboundID,
		InboundNo: inbound.InboundNo,
		Status:    model.StockInboundStatusCancelled,
	}, nil
}

// ApproveStockInbound 审核通过整张入库单，并在同一事务中增加库存和记录流水。
func ApproveStockInbound(ctx context.Context, inboundID int64, reviewerID int64) (*model.ApproveStockInboundResponse, error) {
	if inboundID <= 0 || reviewerID <= 0 {
		return nil, fmt.Errorf("%w：入库单ID或审核人ID错误", ErrInvalidStockInbound)
	}

	tx, err := global.DB.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// 锁定主表记录，防止并发请求重复审核同一张入库单。
	inbound, err := repo.GetStockInboundByIDForUpdate(ctx, tx, inboundID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrStockInboundNotFound
		}
		return nil, err
	}
	if inbound.Status != model.StockInboundStatusPending {
		return nil, ErrStockInboundNotPending
	}

	items, err := repo.ListStockInboundItemsTx(ctx, tx, inboundID)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, ErrStockInboundItemsEmpty
	}

	// 每条SKU明细分别增加可用库存，并写一条入库流水。
	for _, item := range items {
		if err := repo.IncreaseSKUAvailableStock(ctx, tx, item.SKUID, item.Quantity); err != nil {
			return nil, err
		}
		if err := repo.CreateStockInboundLog(ctx, tx, inbound.InboundNo, item.SKUID, item.Quantity, reviewerID); err != nil {
			return nil, err
		}
	}

	reviewedAt := time.Now()
	if err := repo.ApproveStockInbound(ctx, tx, inboundID, reviewerID, reviewedAt); err != nil {
		if errors.Is(err, repo.ErrStockInboundNotEditable) {
			return nil, ErrStockInboundNotPending
		}
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &model.ApproveStockInboundResponse{
		InboundID:  inbound.InboundID,
		InboundNo:  inbound.InboundNo,
		Status:     model.StockInboundStatusCompleted,
		ReviewerID: reviewerID,
		ReviewedAt: reviewedAt,
	}, nil
}

// RejectStockInbound 拒绝待审核入库单，仅更新审核状态和原因。
func RejectStockInbound(ctx context.Context, inboundID int64, reviewerID int64, req *model.RejectStockInboundRequest) (*model.RejectStockInboundResponse, error) {
	if inboundID <= 0 || reviewerID <= 0 || req == nil {
		return nil, fmt.Errorf("%w：入库单ID、审核人ID或请求参数错误", ErrInvalidStockInbound)
	}
	reviewRemark := strings.TrimSpace(req.ReviewRemark)
	if reviewRemark == "" || utf8.RuneCountInString(reviewRemark) > 255 {
		return nil, fmt.Errorf("%w：拒绝原因不能为空且不能超过255个字符", ErrInvalidStockInbound)
	}

	tx, err := global.DB.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// 锁定主表记录，防止与审核通过或重复拒绝并发处理。
	inbound, err := repo.GetStockInboundByIDForUpdate(ctx, tx, inboundID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrStockInboundNotFound
		}
		return nil, err
	}
	if inbound.Status != model.StockInboundStatusPending {
		return nil, ErrStockInboundNotPending
	}

	reviewedAt := time.Now()
	if err := repo.RejectStockInbound(ctx, tx, inboundID, reviewerID, reviewedAt, reviewRemark); err != nil {
		if errors.Is(err, repo.ErrStockInboundNotEditable) {
			return nil, ErrStockInboundNotPending
		}
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &model.RejectStockInboundResponse{
		InboundID:    inbound.InboundID,
		InboundNo:    inbound.InboundNo,
		Status:       model.StockInboundStatusRejected,
		ReviewerID:   reviewerID,
		ReviewedAt:   reviewedAt,
		ReviewRemark: reviewRemark,
	}, nil
}

// ListStockInbounds 查询全部入库申请列表。
func ListStockInbounds(ctx context.Context, req *model.ListStockInboundsRequest) (*model.ListStockInboundsResponse, error) {
	// 设置默认分页参数，并限制单页最大数量。
	page := req.Page
	if page <= 0 {
		page = defaultStockInboundPage
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = defaultStockInboundPageSize
	}
	if pageSize > maxStockInboundPageSize {
		pageSize = maxStockInboundPageSize
	}
	offset := (page - 1) * pageSize

	list, err := repo.ListStockInbounds(ctx, req.Status, pageSize, offset)
	if err != nil {
		return nil, err
	}
	total, err := repo.CountStockInbounds(ctx, req.Status)
	if err != nil {
		return nil, err
	}

	return &model.ListStockInboundsResponse{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// GetStockInboundDetail 查询入库单主信息、全部SKU明细及商品展示信息。
func GetStockInboundDetail(ctx context.Context, inboundID int64, authorization string) (*model.StockInboundDetailResponse, error) {
	if inboundID <= 0 {
		return nil, fmt.Errorf("%w：入库单ID必须大于0", ErrInvalidStockInbound)
	}

	// 1.查询入库单主信息和全部SKU明细。
	inbound, err := repo.GetStockInboundByID(ctx, inboundID) // 根据主键查询入库单信息
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrStockInboundNotFound
		}
		return nil, err
	}
	items, err := repo.ListStockInboundItems(ctx, inboundID) // 查询一张入库单下的全部SKU明细
	if err != nil {
		return nil, err
	}

	// 2.调用商品服务补充SKU信息和商品名称。
	detailItems := make([]model.StockInboundDetailItemResponse, 0, len(items)) // 准备返回给前端的明细列表。
	productNames := make(map[int64]string)                                     // 缓存已查询的商品名称，临时对照表 避免重复调用商品服务。
	for _, item := range items {                                               // 逐条处理这张入库单的SKU明细。
		sku, err := httpclient.GetProductSKU(ctx, item.SKUID, authorization) // 查询当前明细对应的SKU信息。
		if err != nil {
			if errors.Is(err, httpclient.ErrProductSKUNotFound) {
				return nil, fmt.Errorf("%w：%d", ErrProductSKUNotFound, item.SKUID)
			}
			return nil, ErrProductServiceUnavailable
		}
		if sku.SKUID != item.SKUID {
			return nil, ErrProductServiceUnavailable
		} // 返回的SKU ID与明细中的SKU ID不一致时，不继续组装详情。 商品服务接防止口返回错误数据

		productName, exists := productNames[sku.ProductID] // 用这个 SKU 所属的商品 ID，看看临时对照表里有没有对应的商品名称
		if !exists {                                       // exists == false 说明临时对照表里还没有这个商品名称，于是调用接口httpclient.GetProductDetail
			product, err := httpclient.GetProductDetail(ctx, sku.ProductID, authorization) // 查询SKU所属商品的详情。
			if err != nil {
				if errors.Is(err, httpclient.ErrProductNotFound) {
					return nil, fmt.Errorf("%w：%d", ErrProductNotFound, sku.ProductID)
				}
				return nil, ErrProductServiceUnavailable
			}
			if product.ProductID != sku.ProductID {
				return nil, ErrProductServiceUnavailable
			} // 返回的商品ID与SKU所属商品ID不一致时，不继续组装详情。商品服务接防止口返回错误数据
			productName = product.ProductName         // 商品服务返回的商品名称赋给局部变量 productName
			productNames[sku.ProductID] = productName // 把商品名称存进临时对照表，供后续 SKU 复
		}

		detailItems = append(detailItems, model.StockInboundDetailItemResponse{ // 组合库存明细与商品信息，加入返回列表。
			ItemID:      item.ItemID,
			SKUID:       item.SKUID,
			ProductID:   sku.ProductID,
			ProductName: productName,
			SKUName:     sku.SKUName,
			SKUImage:    sku.SKUImage,
			Quantity:    item.Quantity,
			CostPrice:   item.CostPrice,
		})
	}

	// 3.组合入库单主信息和全部明细。
	return &model.StockInboundDetailResponse{
		InboundID:    inbound.InboundID,
		InboundNo:    inbound.InboundNo,
		SupplierName: inbound.SupplierName,
		Status:       inbound.Status,
		OperatorID:   inbound.OperatorID,
		ReviewerID:   inbound.ReviewerID,
		SubmittedAt:  inbound.SubmittedAt,
		ReviewedAt:   inbound.ReviewedAt,
		Remark:       inbound.Remark,
		ReviewRemark: inbound.ReviewRemark,
		CreatedAt:    inbound.CreatedAt,
		UpdatedAt:    inbound.UpdatedAt,
		Items:        detailItems,
	}, nil
}

// generateUniqueInboundNo 生成 IN20260914-A8K2 格式的入库单号。
func generateUniqueInboundNo(ctx context.Context) (string, error) {
	for i := 0; i < inboundNoGenerateAttempts; i++ {
		code, err := randomString(4)
		if err != nil {
			return "", err
		}

		inboundNo := fmt.Sprintf("IN%s-%s", time.Now().Format("20060102"), code)
		exists, err := repo.ExistsStockInboundNo(ctx, inboundNo)
		if err != nil {
			return "", err
		}
		if !exists { // 如果这个入库单号数据库里不存在
			return inboundNo, nil // 返回生成好的单号
		}
	}

	return "", errors.New("入库单号生成失败，请重试")
}

// randomString 使用安全随机数生成大写字母和数字组合。
func randomString(length int) (string, error) {
	const letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	result := make([]byte, length)
	max := big.NewInt(int64(len(letters)))
	for i := range result {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		result[i] = letters[n.Int64()]
	}

	return string(result), nil
}
