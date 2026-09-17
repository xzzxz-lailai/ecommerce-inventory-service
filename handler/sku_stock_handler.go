package handler

import (
	"net/http"
	"strconv"

	"inventory-service/model"
	"inventory-service/pkg"
	"inventory-service/service"

	"github.com/gin-gonic/gin"
)

// ListSKUStocks 供公司内部账号分页查看已有的 SKU 库存记录。
func ListSKUStocks(c *gin.Context) {
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

	resp, err := service.ListSKUStocks(c.Request.Context(), &model.ListSKUStocksRequest{
		Page: page, PageSize: pageSize,
	})
	if err != nil {
		pkg.Error(c, http.StatusInternalServerError, "SKU库存列表查询失败")
		return
	}
	pkg.Success(c, "SKU库存列表查询成功", resp)
}

// GetSKUStockBalance 供公司内部账号查看单个 SKU 的当前库存余额。
func GetSKUStockBalance(c *gin.Context) {
	skuID, err := strconv.ParseInt(c.Param("sku_id"), 10, 64)
	if err != nil || skuID <= 0 {
		pkg.Error(c, http.StatusBadRequest, "SKU ID错误")
		return
	}

	resp, err := service.GetSKUStockBalance(c.Request.Context(), skuID)
	if err != nil {
		pkg.Error(c, http.StatusInternalServerError, "SKU库存余额查询失败")
		return
	}
	pkg.Success(c, "SKU库存余额查询成功", resp)
}
