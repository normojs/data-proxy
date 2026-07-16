package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

type modelTokenPackageSkuRequest struct {
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	Models          []string `json:"models"`
	TotalTokens     int64    `json:"total_tokens"`
	InputRatio      *float64 `json:"input_ratio"`
	OutputRatio     *float64 `json:"output_ratio"`
	CacheRatio      *float64 `json:"cache_ratio"`
	Priority        int      `json:"priority"`
	DurationSeconds int64    `json:"duration_seconds"`
	PriceQuota      int      `json:"price_quota"`
	Status          string   `json:"status"`
	SortOrder       int      `json:"sort_order"`
}

func skuInputFromRequest(req modelTokenPackageSkuRequest, createdBy int) model.ModelTokenPackageSkuInput {
	inputRatio, outputRatio, cacheRatio := 1.0, 1.0, 1.0
	if req.InputRatio != nil {
		inputRatio = *req.InputRatio
	}
	if req.OutputRatio != nil {
		outputRatio = *req.OutputRatio
	}
	if req.CacheRatio != nil {
		cacheRatio = *req.CacheRatio
	}
	return model.ModelTokenPackageSkuInput{
		Name:            strings.TrimSpace(req.Name),
		Description:     strings.TrimSpace(req.Description),
		Models:          req.Models,
		TotalTokens:     req.TotalTokens,
		InputRatio:      inputRatio,
		OutputRatio:     outputRatio,
		CacheRatio:      cacheRatio,
		Priority:        req.Priority,
		DurationSeconds: req.DurationSeconds,
		PriceQuota:      req.PriceQuota,
		Status:          req.Status,
		SortOrder:       req.SortOrder,
		CreatedBy:       createdBy,
	}
}

// ListPublicModelTokenPackageSkus lists enabled SKUs for self-service purchase.
func ListPublicModelTokenPackageSkus(c *gin.Context) {
	rows, err := model.ListModelTokenPackageSkus(true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rows)
}

// PurchaseModelTokenPackageSku spends wallet quota and grants a package.
func PurchaseModelTokenPackageSku(c *gin.Context) {
	userId := c.GetInt("id")
	skuId, err := strconv.Atoi(c.Param("id"))
	if err != nil || skuId <= 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid sku id"})
		return
	}
	pkg, sku, err := model.PurchaseModelTokenPackageSkuWithWallet(userId, skuId)
	if err != nil {
		switch {
		case errors.Is(err, model.ErrModelTokenPackageSkuNotFound):
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "package sku not found"})
		case errors.Is(err, model.ErrModelTokenPackageSkuDisabled):
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "package sku is disabled"})
		case errors.Is(err, model.ErrModelTokenPackageSkuInsufficientQ):
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Wallet balance is insufficient for this package. Please top up your wallet first.",
				"code":    "insufficient_user_quota",
			})
		default:
			common.ApiError(c, err)
		}
		return
	}
	common.ApiSuccess(c, gin.H{
		"package": pkg,
		"sku":     sku,
	})
}

// AdminListModelTokenPackageSkus lists all SKUs for operators.
func AdminListModelTokenPackageSkus(c *gin.Context) {
	enabledOnly := c.Query("enabled_only") == "true"
	rows, err := model.ListModelTokenPackageSkus(enabledOnly)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rows)
}

// AdminCreateModelTokenPackageSku creates a sellable package SKU.
func AdminCreateModelTokenPackageSku(c *gin.Context) {
	var req modelTokenPackageSkuRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	sku, err := model.CreateModelTokenPackageSku(skuInputFromRequest(req, c.GetInt("id")))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	model.RecordLogWithAdminInfo(c.GetInt("id"), model.LogTypeManage,
		"create model token package sku id="+strconv.Itoa(sku.Id)+" tokens="+strconv.FormatInt(sku.TotalTokens, 10),
		map[string]interface{}{
			"sku_id":       sku.Id,
			"total_tokens": sku.TotalTokens,
			"price_quota":  sku.PriceQuota,
			"models":       sku.Models,
		})
	common.ApiSuccess(c, sku)
}

// AdminUpdateModelTokenPackageSku updates an existing SKU.
func AdminUpdateModelTokenPackageSku(c *gin.Context) {
	skuId, err := strconv.Atoi(c.Param("id"))
	if err != nil || skuId <= 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid sku id"})
		return
	}
	var req modelTokenPackageSkuRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	sku, err := model.UpdateModelTokenPackageSku(skuId, skuInputFromRequest(req, c.GetInt("id")))
	if err != nil {
		if errors.Is(err, model.ErrModelTokenPackageSkuNotFound) {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "package sku not found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	model.RecordLogWithAdminInfo(c.GetInt("id"), model.LogTypeManage,
		"update model token package sku id="+strconv.Itoa(sku.Id),
		map[string]interface{}{
			"sku_id":       sku.Id,
			"total_tokens": sku.TotalTokens,
			"price_quota":  sku.PriceQuota,
			"status":       sku.Status,
		})
	common.ApiSuccess(c, sku)
}
