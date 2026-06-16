package controller

import (
	"context"
	"net/http"

	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

type FetchExchangeRateRequest struct {
	CurrencyCode string `json:"currency_code"`
}

func FetchExchangeRate(c *gin.Context) {
	var req FetchExchangeRateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}
	result, err := service.UpdateUSDExchangeRateFromProvider(context.Background(), req.CurrencyCode)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    result,
	})
}
