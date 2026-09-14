package middleware

import (
	"strings"

	"inventory-service/pkg"

	"github.com/gin-gonic/gin"
)

const (
	roleAdminID    int8 = 1
	roleOperatorID int8 = 2
)

// Authorization 解析 Authorization 请求头，并把登录主体信息写入上下文。
func Authorization() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !setClaimsToContext(c) {
			return
		}
		c.Next()
	}
}

// RequireAdmin 只允许管理员内部账号访问。
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !setClaimsToContext(c) {
			return
		}

		roleIDValue, exists := c.Get("roleID")
		roleID, ok := roleIDValue.(int8)
		if !exists || !ok || c.GetString("subjectType") != "user" || roleID != roleAdminID {
			pkg.Error(c, 403, "无权限访问")
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireUser 只允许公司内部账号访问。
func RequireUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !setClaimsToContext(c) {
			return
		}

		if c.GetString("subjectType") != "user" {
			pkg.Error(c, 403, "无权限访问")
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireOperator 只允许运营内部账号访问。
func RequireOperator() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !setClaimsToContext(c) {
			return
		}

		roleIDValue, exists := c.Get("roleID")
		roleID, ok := roleIDValue.(int8)
		if !exists || !ok || c.GetString("subjectType") != "user" || roleID != roleOperatorID {
			pkg.Error(c, 403, "无权限访问")
			c.Abort()
			return
		}

		c.Next()
	}
}

func setClaimsToContext(c *gin.Context) bool {
	authorization := c.GetHeader("Authorization")
	parts := strings.SplitN(authorization, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" || parts[1] == "" {
		pkg.Error(c, 401, "请先登录")
		c.Abort()
		return false
	}

	claims, err := pkg.ParseToken(parts[1])
	if err != nil {
		pkg.Error(c, 401, "请先登录")
		c.Abort()
		return false
	}

	c.Set("subjectType", claims.SubjectType)
	c.Set("userID", claims.UserID)
	c.Set("roleID", claims.RoleID)
	c.Set("partnerID", claims.PartnerID)
	c.Set("partnerUserID", claims.PartnerUserID)

	return true
}
