package handler

import (
	"errors"
	"net/http"

	"inventory-service/model"
	"inventory-service/pkg"
	"inventory-service/service"

	"github.com/gin-gonic/gin"
)

// CreateStockInbound 创建采购入库申请。
func CreateStockInbound(c *gin.Context) {
	var req model.CreateStockInboundRequest        // 定义创建入库申请的请求参数。
	if err := c.ShouldBindJSON(&req); err != nil { // 接收并校验前端提交的 JSON 参数。
		pkg.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	// 从登录信息中取得运营人员ID，并调用业务层创建申请。
	resp, err := service.CreateStockInbound(c.Request.Context(), &req, c.GetInt64("userID"), c.GetHeader("Authorization"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidStockInbound): // 入库参数不符合业务规则。
			pkg.Error(c, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrProductSKUNotFound): // 商品服务中不存在对应的SKU。
			pkg.Error(c, http.StatusNotFound, err.Error())
		case errors.Is(err, service.ErrProductServiceUnavailable): // 商品服务当前无法正常访问。
			pkg.Error(c, http.StatusServiceUnavailable, err.Error())
		default:
			pkg.Error(c, http.StatusInternalServerError, "入库申请创建失败")
		}
		return
	}

	pkg.Success(c, "入库申请创建成功，等待审核", resp)
}
