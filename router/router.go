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
			// 运营创建采购入库单草稿 (一张入库单草稿只对应一个供应商)
			admin.POST("/stock/inbounds", middleware.RequireOperator(), handler.CreateStockInbound)
			// 公司内部账号查询全部采购入库申请列表 (一个入库单里面可以有多个sku明细)
			admin.GET("/stock/inbounds", middleware.RequireUser(), handler.ListStockInbounds)
			// 公司内部账号查询一张采购入库申请及其全部SKU明细
			admin.GET("/stock/inbounds/:inbound_id", middleware.RequireUser(), handler.GetStockInboundDetail)
			// 运营向采购入库单草稿添加SKU明细，每次请求添加一个SKU明细(每次调用接口添加一条 SKU 明细,可以连续调用多次，添加多个不同 SKU)
			admin.POST("/stock/inbounds/:inbound_id/items", middleware.RequireOperator(), handler.CreateStockInboundItem)
			// 运营提交采购入库申请，提交后等待管理员审核 (提交的是整张入库单，其中包含该入库单下的全部 SKU 明细，不是只提交某一条明细)
			admin.POST("/stock/inbounds/:inbound_id/submit", middleware.RequireOperator(), handler.SubmitStockInbound)
		}
	}

	return r
}
