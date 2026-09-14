package service

import (
	"context"
	"crypto/rand"
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
	ErrProductSKUNotFound        = errors.New("SKU不存在")
	ErrProductServiceUnavailable = errors.New("商品服务暂时不可用")
)

const inboundNoGenerateAttempts = 5

// CreateStockInbound 创建一张待审核的采购入库单。
func CreateStockInbound(ctx context.Context, req *model.CreateStockInboundRequest, operatorID int64, authorization string) (*model.CreateStockInboundResponse, error) {
	// 1.校验基础参数
	supplierName := strings.TrimSpace(req.SupplierName) // 字符串首尾的空白字符去掉
	if supplierName == "" {
		return nil, fmt.Errorf("%w：供应商名称不能为空", ErrInvalidStockInbound)
	}
	if operatorID <= 0 {
		return nil, fmt.Errorf("%w：运营人员ID不能为空", ErrInvalidStockInbound)
	}
	if len(req.Items) == 0 {
		return nil, fmt.Errorf("%w：至少提交一条SKU明细", ErrInvalidStockInbound)
	}

	// 2.遍历所有入库明细
	// 检查每一条入库明细的数据是否合法
	// 防止同一次申请中重复提交同一个 SKU
	seenSKUs := make(map[int64]struct{}, len(req.Items)) //临时SKU清单
	for _, item := range req.Items {
		if item.SKUID <= 0 {
			return nil, fmt.Errorf("%w：SKU ID必须大于0", ErrInvalidStockInbound)
		}
		if item.Quantity <= 0 {
			return nil, fmt.Errorf("%w：入库数量必须大于0", ErrInvalidStockInbound)
		}
		if item.CostPrice < 0 {
			return nil, fmt.Errorf("%w：采购成本不能小于0", ErrInvalidStockInbound)
		}
		if _, exists := seenSKUs[item.SKUID]; exists { // 用于检查重复 SKU
			return nil, fmt.Errorf("%w：SKU %d重复", ErrInvalidStockInbound, item.SKUID)
		}
		seenSKUs[item.SKUID] = struct{}{} // 记录当前这个 SKUID 已经出现过了,把这个 SKU ID 记到 seenSKUs 里面，后面用来判断有没有重复

		// 3.调用商品服务——公司内部账号查询 SKU 详情接口 (外部服务调用放在事务前，避免长时间占用数据库连接和事务)
		// 一次 HTTP 请求只能查询一个 sku_id
		sku, err := httpclient.GetProductSKU(ctx, item.SKUID, authorization)
		if err != nil {
			switch {
			case errors.Is(err, httpclient.ErrProductSKUNotFound):
				return nil, fmt.Errorf("%w：%d", ErrProductSKUNotFound, item.SKUID)
			default:
				return nil, ErrProductServiceUnavailable
			}
		}
		if sku.SKUID != item.SKUID {
			return nil, ErrProductServiceUnavailable
		}
	}
	// 4.生成入库单号   同一次申请中的所有 SKU 使用同一个 inbound_no
	inboundNo, err := generateUniqueInboundNo(ctx)
	if err != nil {
		return nil, err
	}
	// 5.组装待写入的明细列表
	inbounds := make([]*model.StockInbound, 0, len(req.Items)) // 创建一个“等待写入数据库的入库明细列表”
	for _, item := range req.Items {
		inbounds = append(inbounds, &model.StockInbound{ // 通过 append 添加到 inbounds 列表中
			InboundNo:    inboundNo,
			SupplierName: supplierName,
			SKUID:        item.SKUID,
			Quantity:     item.Quantity,
			CostPrice:    item.CostPrice,
			Status:       model.StockInboundStatusPending,
			OperatorID:   operatorID,
			Remark:       req.Remark,
		})
	}

	tx, err := global.DB.BeginTxx(ctx, nil) // 开启事务
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	// 6.把inbounds入库明细信息,写入到数据库stock_inbounds表中
	if err := repo.CreateStockInbounds(ctx, tx, inbounds); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	// 7.封装明细列表，用于返回给前端
	items := make([]model.CreateStockInboundItemResponse, 0, len(inbounds))
	for _, inbound := range inbounds {
		items = append(items, model.CreateStockInboundItemResponse{
			InboundID: inbound.InboundID,
			SKUID:     inbound.SKUID,
			Quantity:  inbound.Quantity,
			CostPrice: inbound.CostPrice,
		})
	}
	// 8.返回创建结果
	return &model.CreateStockInboundResponse{
		InboundNo: inboundNo,
		Status:    model.StockInboundStatusPending,
		Items:     items,
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
		if !exists {
			return inboundNo, nil
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
