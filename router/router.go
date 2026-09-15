package router

import (
	"inventory-service/handler"
	"inventory-service/middleware"

	"github.com/gin-gonic/gin"
)

// Router 创建库存服务路由。
func Router() *gin.Engine {
	r := gin.Default()

	api := r.Group("/api/v1")
	{
		admin := api.Group("/admin")
		{
			// 运营创建采购入库单草稿
			admin.POST("/stock/inbounds", middleware.RequireOperator(), handler.CreateStockInbound)
			// 运营向采购入库单草稿添加SKU明细，每次请求添加一个SKU
			admin.POST("/stock/inbounds/:inbound_id/items", middleware.RequireOperator(), handler.CreateStockInboundItem)
			// 运营提交采购入库申请，提交后等待管理员审核
			admin.POST("/stock/inbounds/:inbound_id/submit", middleware.RequireOperator(), handler.SubmitStockInbound)
		}
	}

	return r
}
