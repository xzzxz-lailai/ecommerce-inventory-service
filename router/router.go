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
			// 运营创建采购入库申请，创建后等待管理员审核。
			admin.POST("/stock/inbounds", middleware.RequireOperator(), handler.CreateStockInbound)
		}
	}

	return r
}
