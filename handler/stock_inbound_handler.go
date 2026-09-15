package handler

import (
	"errors"
	"net/http"
	"strconv"

	"inventory-service/model"
	"inventory-service/pkg"
	"inventory-service/service"

	"github.com/gin-gonic/gin"
)

// CreateStockInbound 创建采购入库单草稿。
func CreateStockInbound(c *gin.Context) {
	var req model.CreateStockInboundRequest        // 定义创建入库单草稿的请求参数。
	if err := c.ShouldBindJSON(&req); err != nil { // 接收并校验前端提交的 JSON 参数。
		pkg.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	// 从登录信息中取得运营人员ID，并调用业务层创建草稿。
	resp, err := service.CreateStockInbound(c.Request.Context(), &req, c.GetInt64("userID"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidStockInbound): // 入库单参数不符合业务规则。
			pkg.Error(c, http.StatusBadRequest, err.Error())
		default:
			pkg.Error(c, http.StatusInternalServerError, "入库单草稿创建失败")
		}
		return
	}

	pkg.Success(c, "入库单草稿创建成功", resp)
}

// CreateStockInboundItem 向采购入库单草稿添加SKU明细。
func CreateStockInboundItem(c *gin.Context) {
	inboundID, err := strconv.ParseInt(c.Param("inbound_id"), 10, 64)
	if err != nil || inboundID <= 0 {
		pkg.Error(c, http.StatusBadRequest, "入库单ID错误")
		return
	}

	var req model.CreateStockInboundItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Error(c, http.StatusBadRequest, "参数错误")
		return
	}

	resp, err := service.CreateStockInboundItem(
		c.Request.Context(),
		inboundID,
		&req,
		c.GetInt64("userID"),
		c.GetHeader("Authorization"),
	)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidStockInbound):
			pkg.Error(c, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrStockInboundNotFound), errors.Is(err, service.ErrProductSKUNotFound):
			pkg.Error(c, http.StatusNotFound, err.Error())
		case errors.Is(err, service.ErrStockInboundForbidden):
			pkg.Error(c, http.StatusForbidden, err.Error())
		case errors.Is(err, service.ErrStockInboundNotDraft), errors.Is(err, service.ErrStockInboundItemDuplicate):
			pkg.Error(c, http.StatusConflict, err.Error())
		case errors.Is(err, service.ErrProductServiceUnavailable):
			pkg.Error(c, http.StatusServiceUnavailable, err.Error())
		default:
			pkg.Error(c, http.StatusInternalServerError, "入库明细添加失败")
		}
		return
	}

	pkg.Success(c, "入库明细添加成功", resp)
}

// SubmitStockInbound 提交采购入库申请，等待管理员审核。
func SubmitStockInbound(c *gin.Context) {
	inboundID, err := strconv.ParseInt(c.Param("inbound_id"), 10, 64)
	if err != nil || inboundID <= 0 {
		pkg.Error(c, http.StatusBadRequest, "入库单ID错误")
		return
	}

	resp, err := service.SubmitStockInbound(c.Request.Context(), inboundID, c.GetInt64("userID"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidStockInbound), errors.Is(err, service.ErrStockInboundItemsEmpty):
			pkg.Error(c, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrStockInboundNotFound):
			pkg.Error(c, http.StatusNotFound, err.Error())
		case errors.Is(err, service.ErrStockInboundForbidden):
			pkg.Error(c, http.StatusForbidden, err.Error())
		case errors.Is(err, service.ErrStockInboundNotDraft):
			pkg.Error(c, http.StatusConflict, err.Error())
		default:
			pkg.Error(c, http.StatusInternalServerError, "入库申请提交失败")
		}
		return
	}

	pkg.Success(c, "入库申请提交成功，等待审核", resp)
}
