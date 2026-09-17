package service

import (
	"context"
	"database/sql"
	"errors"

	"inventory-service/model"
	"inventory-service/repo"
)

const (
	defaultSKUStockPage     = 1
	defaultSKUStockPageSize = 10
	maxSKUStockPageSize     = 100
)

// GetSKUStockBalance 查询后台展示用的库存余额；无库存记录时按零库存返回。
func GetSKUStockBalance(ctx context.Context, skuID int64) (*model.SKUStockBalanceResponse, error) {
	stock, err := repo.GetSKUStockByID(ctx, skuID)
	if errors.Is(err, sql.ErrNoRows) {
		// 尚未建立库存记录不等于商品服务中一定存在该 SKU。
		return &model.SKUStockBalanceResponse{SKUID: skuID}, nil
	}
	if err != nil {
		return nil, err
	}
	return &model.SKUStockBalanceResponse{
		SKUID:          stock.SKUID,
		AvailableStock: stock.AvailableStock,
		LockedStock:    stock.LockedStock,
		UpdatedAt:      &stock.UpdatedAt,
	}, nil
}

// ListSKUStocks 分页查询库存表中已有的 SKU 库存记录。
func ListSKUStocks(ctx context.Context, req *model.ListSKUStocksRequest) (*model.ListSKUStocksResponse, error) {
	page := req.Page
	if page <= 0 {
		page = defaultSKUStockPage
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = defaultSKUStockPageSize
	}
	if pageSize > maxSKUStockPageSize {
		pageSize = maxSKUStockPageSize
	}
	offset := (page - 1) * pageSize

	list, err := repo.ListSKUStocks(ctx, pageSize, offset)
	if err != nil {
		return nil, err
	}
	total, err := repo.CountSKUStocks(ctx)
	if err != nil {
		return nil, err
	}
	return &model.ListSKUStocksResponse{
		List: list, Total: total, Page: page, PageSize: pageSize,
	}, nil
}
