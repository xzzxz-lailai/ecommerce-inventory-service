package repo

import (
	"context"

	"inventory-service/global"
	"inventory-service/model"
)

// ListStockLogs 按可选条件查询库存流水，最新的流水排在前面。(四个筛选条件)
func ListStockLogs(ctx context.Context, skuID *int64, businessType *int8, limit, offset int) ([]model.StockLogListItem, error) {
	// 不筛选条件,倒叙查询库存流水表全部流水
	query := `SELECT log_id, sku_id, business_type, business_no,
		available_change, locked_change, operator_id, remark, created_at
		FROM stock_logs
		ORDER BY log_id DESC
		LIMIT ? OFFSET ?`
	args := []interface{}{limit, offset}

	// 根据是否传入两个筛选条件，选择对应的完整SQL。
	switch {
	case skuID != nil && businessType != nil: // 同时按SKU和业务类型筛选。
		query = `SELECT log_id, sku_id, business_type, business_no,
			available_change, locked_change, operator_id, remark, created_at
			FROM stock_logs
			WHERE sku_id = ? AND business_type = ?
			ORDER BY log_id DESC
			LIMIT ? OFFSET ?`
		args = []interface{}{*skuID, *businessType, limit, offset}
	case skuID != nil: // 只按SKU筛选。
		query = `SELECT log_id, sku_id, business_type, business_no,
			available_change, locked_change, operator_id, remark, created_at
			FROM stock_logs
			WHERE sku_id = ?
			ORDER BY log_id DESC
			LIMIT ? OFFSET ?`
		args = []interface{}{*skuID, limit, offset}
	case businessType != nil: // 只按业务类型筛选。
		query = `SELECT log_id, sku_id, business_type, business_no,
			available_change, locked_change, operator_id, remark, created_at
			FROM stock_logs
			WHERE business_type = ?
			ORDER BY log_id DESC
			LIMIT ? OFFSET ?`
		args = []interface{}{*businessType, limit, offset}
	}

	list := make([]model.StockLogListItem, 0)
	if err := global.DB.SelectContext(ctx, &list, query, args...); err != nil {
		return nil, err
	}
	return list, nil
}

// CountStockLogs 查询相同筛选条件下的库存流水总条数。
func CountStockLogs(ctx context.Context, skuID *int64, businessType *int8) (int64, error) {
	// 默认统计全部库存流水。
	query := `SELECT COUNT(*) FROM stock_logs`
	args := []interface{}{}

	// 总数查询使用与列表查询相同的筛选条件。
	switch {
	case skuID != nil && businessType != nil: // 统计指定SKU、指定业务类型的流水。
		query = `SELECT COUNT(*) FROM stock_logs WHERE sku_id = ? AND business_type = ?`
		args = []interface{}{*skuID, *businessType}
	case skuID != nil: // 只统计指定SKU的流水。
		query = `SELECT COUNT(*) FROM stock_logs WHERE sku_id = ?`
		args = []interface{}{*skuID}
	case businessType != nil: // 只统计指定业务类型的流水。
		query = `SELECT COUNT(*) FROM stock_logs WHERE business_type = ?`
		args = []interface{}{*businessType}
	}

	var total int64
	if err := global.DB.GetContext(ctx, &total, query, args...); err != nil {
		return 0, err
	}
	return total, nil
}
