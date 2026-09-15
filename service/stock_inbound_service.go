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

	"inventory-service/global"
	"inventory-service/httpclient"
	"inventory-service/model"
	"inventory-service/repo"
)

var (
	ErrInvalidStockInbound       = errors.New("入库申请参数错误")
	ErrStockInboundNotFound      = errors.New("入库单不存在")
	ErrStockInboundForbidden     = errors.New("无权操作该入库单")
	ErrStockInboundNotDraft      = errors.New("只有草稿状态的入库单可以添加明细")
	ErrStockInboundItemsEmpty    = errors.New("入库单至少需要一条SKU明细")
	ErrStockInboundItemDuplicate = errors.New("该入库单中已经存在此SKU")
	ErrProductSKUNotFound        = errors.New("SKU不存在")
	ErrProductServiceUnavailable = errors.New("商品服务暂时不可用")
)

const inboundNoGenerateAttempts = 5

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
	// 商品服务返回的 SKUID和前端填入的skuid是否一致,防止商品服务接口返回错误数据
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
