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

// ListStockInbounds 查询全部入库申请列表。
func ListStockInbounds(c *gin.Context) {
	var status *int8 // // 接收可选的状态参数；nil表示前端未传status，需要查询全部状态
	if statusValue := c.Query("status"); statusValue != "" {
		parsedStatus, err := strconv.ParseInt(statusValue, 10, 8) // 把前端传来的字符串status转成int64类型
		// 如果字符串转数字失败，或者状态值小于最小值，或者状态值大于最大值，就认为这个状态参数不合法
		if err != nil || parsedStatus < int64(model.StockInboundStatusDraft) || parsedStatus > int64(model.StockInboundStatusCancelled) {
			pkg.Error(c, http.StatusBadRequest, "入库单状态错误")
			return
		}
		stockInboundStatus := int8(parsedStatus) // 把转换后的数字保存成 int8类型
		status = &stockInboundStatus             //取status指针,如果前端没传入任何值,就是nil 代表查询全部
	}

	page, ok := parsePositiveStockInboundQuery(c, "page")
	if !ok {
		pkg.Error(c, http.StatusBadRequest, "页码错误")
		return
	}
	pageSize, ok := parsePositiveStockInboundQuery(c, "page_size")
	if !ok {
		pkg.Error(c, http.StatusBadRequest, "每页数量错误")
		return
	}

	resp, err := service.ListStockInbounds(c.Request.Context(), &model.ListStockInboundsRequest{
		Status:   status,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		pkg.Error(c, http.StatusInternalServerError, "入库申请列表查询失败")
		return
	}

	pkg.Success(c, "入库申请列表查询成功", resp)
}

// GetStockInboundDetail 查询一张入库申请的完整信息和全部SKU明细。
func GetStockInboundDetail(c *gin.Context) {
	inboundID, err := strconv.ParseInt(c.Param("inbound_id"), 10, 64)
	if err != nil || inboundID <= 0 {
		pkg.Error(c, http.StatusBadRequest, "入库单ID错误")
		return
	}

	resp, err := service.GetStockInboundDetail(
		c.Request.Context(),
		inboundID,
		c.GetHeader("Authorization"),
	)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidStockInbound):
			pkg.Error(c, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrStockInboundNotFound), errors.Is(err, service.ErrProductSKUNotFound), errors.Is(err, service.ErrProductNotFound):
			pkg.Error(c, http.StatusNotFound, err.Error())
		case errors.Is(err, service.ErrProductServiceUnavailable):
			pkg.Error(c, http.StatusServiceUnavailable, err.Error())
		default:
			pkg.Error(c, http.StatusInternalServerError, "入库申请详情查询失败")
		}
		return
	}

	pkg.Success(c, "入库申请详情查询成功", resp)
}

// parsePositiveStockInboundQuery 解析可选的正整数查询参数，未传时返回0。
func parsePositiveStockInboundQuery(c *gin.Context, key string) (int, bool) {
	value := c.Query(key)
	if value == "" {
		return 0, true
	}

	parsedValue, err := strconv.Atoi(value)
	if err != nil || parsedValue <= 0 {
		return 0, false
	}
	return parsedValue, true
}
