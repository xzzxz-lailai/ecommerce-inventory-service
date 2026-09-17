package repo

import (
	"context"

	"inventory-service/global"
	"inventory-service/model"
)

// GetSKUStockByID 根据 SKU ID 查询库存余额记录。
func GetSKUStockByID(ctx context.Context, skuID int64) (*model.SKUStock, error) {
	var stock model.SKUStock
	err := global.DB.GetContext(ctx, &stock,
		`SELECT sku_id, available_stock, locked_stock, created_at, updated_at
		FROM sku_stocks WHERE sku_id = ?`, skuID)
	if err != nil {
		return nil, err
	}
	return &stock, nil
}

// ListSKUStocks 分页查询已有库存记录，最近变化的排在前面。
func ListSKUStocks(ctx context.Context, limit, offset int) ([]model.SKUStockListItem, error) {
	query := `SELECT sku_id, available_stock, locked_stock, updated_at
		FROM sku_stocks
		ORDER BY updated_at DESC, sku_id DESC
		LIMIT ? OFFSET ?`

	list := make([]model.SKUStockListItem, 0)
	if err := global.DB.SelectContext(ctx, &list, query, limit, offset); err != nil {
		return nil, err
	}
	return list, nil
}

// CountSKUStocks 查询已有库存记录的总数。
func CountSKUStocks(ctx context.Context) (int64, error) {
	query := `SELECT COUNT(*) FROM sku_stocks`

	var total int64
	if err := global.DB.GetContext(ctx, &total, query); err != nil {
		return 0, err
	}
	return total, nil
}
