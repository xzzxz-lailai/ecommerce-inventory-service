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
	ErrProductNotFound           = errors.New("商品不存在")
	ErrProductServiceUnavailable = errors.New("商品服务暂时不可用")
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
		InboundAt:    inbound.InboundAt,
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
