package handler

import (
	"net/http"
	"strconv"

	"inventory-service/model"
	"inventory-service/pkg"
	"inventory-service/service"

	"github.com/gin-gonic/gin"
)

// ListStockLogs 查询公司内部可见的库存流水列表。
func ListStockLogs(c *gin.Context) {
	var skuID *int64
	if value, exists := c.GetQuery("sku_id"); exists {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil || parsed <= 0 {
			pkg.Error(c, http.StatusBadRequest, "SKU ID错误")
			return
		}
		skuID = &parsed
	}

	var businessType *int8
	if value, exists := c.GetQuery("business_type"); exists {
		parsed, err := strconv.ParseInt(value, 10, 8)
		if err != nil || parsed < int64(model.StockBusinessTypeInbound) || parsed > int64(model.StockBusinessTypeOrderRelease) {
			pkg.Error(c, http.StatusBadRequest, "库存流水业务类型错误")
			return
		}
		typed := int8(parsed)
		businessType = &typed
	}

	page, ok := parsePositiveQuery(c, "page")
	if !ok {
		pkg.Error(c, http.StatusBadRequest, "页码错误")
		return
	}
	pageSize, ok := parsePositiveQuery(c, "page_size")
	if !ok {
		pkg.Error(c, http.StatusBadRequest, "每页数量错误")
		return
	}

	resp, err := service.ListStockLogs(c.Request.Context(), &model.ListStockLogsRequest{
		SKUID: skuID, BusinessType: businessType, Page: page, PageSize: pageSize,
	})
	if err != nil {
		pkg.Error(c, http.StatusInternalServerError, "库存流水列表查询失败")
		return
	}
	pkg.Success(c, "库存流水列表查询成功", resp)
}
