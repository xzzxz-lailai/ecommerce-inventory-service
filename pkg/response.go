package pkg

import "github.com/gin-gonic/gin"

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func Success(c *gin.Context, message string, data interface{}) {
	c.JSON(200, Response{Code: 0, Message: message, Data: data})
}

func Error(c *gin.Context, httpCode int, message string) {
	c.JSON(httpCode, Response{Code: 1, Message: message, Data: nil})
}

func ErrorWithData(c *gin.Context, httpCode int, message string, data interface{}) {
	c.JSON(httpCode, Response{Code: 1, Message: message, Data: data})
}
