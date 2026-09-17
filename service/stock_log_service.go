package service

import (
	"context"

	"inventory-service/model"
	"inventory-service/repo"
)

const (
	defaultStockLogPage     = 1
	defaultStockLogPageSize = 10
	maxStockLogPageSize     = 100
)

// ListStockLogs 分页查询后台库存流水，可按 SKU 和业务类型筛选。
func ListStockLogs(ctx context.Context, req *model.ListStockLogsRequest) (*model.ListStockLogsResponse, error) {
	// 未传分页参数时使用默认值，并限制每页最多返回100条。
	page := req.Page
	if page <= 0 {
		page = defaultStockLogPage
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = defaultStockLogPageSize
	}
	if pageSize > maxStockLogPageSize {
		pageSize = maxStockLogPageSize
	}
	offset := (page - 1) * pageSize

	// 查询当前页流水，再统计相同筛选条件下的总条数。
	list, err := repo.ListStockLogs(ctx, req.SKUID, req.BusinessType, pageSize, offset)
	if err != nil {
		return nil, err
	}
	total, err := repo.CountStockLogs(ctx, req.SKUID, req.BusinessType)
	if err != nil {
		return nil, err
	}
	// 把列表和分页信息一起返回给前端。
	return &model.ListStockLogsResponse{
		List: list, Total: total, Page: page, PageSize: pageSize,
	}, nil
}
