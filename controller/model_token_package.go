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

type modelTokenPackageGrantRequest struct {
	Name        string   `json:"name"`
	Models      []string `json:"models"`
	TotalTokens int64    `json:"total_tokens"`
	InputRatio  *float64 `json:"input_ratio"`
	OutputRatio *float64 `json:"output_ratio"`
	CacheRatio  *float64 `json:"cache_ratio"`
	Priority    int      `json:"priority"`
	ExpiredAt   int64    `json:"expired_at"`
	Remark      string   `json:"remark"`
}

type modelTokenPackageAdjustRequest struct {
	Delta  int64  `json:"delta"`
	Reason string `json:"reason"`
}

func GetSelfModelTokenPackages(c *gin.Context) {
	userId := c.GetInt("id")
	includeInactive := c.Query("include_inactive") == "true"
	rows, err := model.ListModelTokenPackagesByUser(userId, includeInactive)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rows)
}

func GetSelfModelTokenPackageLedger(c *gin.Context) {
	userId := c.GetInt("id")
	packageId, err := strconv.Atoi(c.Param("id"))
	if err != nil || packageId <= 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid package id"})
		return
	}
	pkg, err := model.GetModelTokenPackageById(packageId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if pkg.UserId != userId {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "package not found"})
		return
	}
	pageInfo := common.GetPageQuery(c)
	rows, total, err := model.ListModelTokenPackageLedger(packageId, userId, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(rows)
	common.ApiSuccess(c, pageInfo)
}

func AdminListUserModelTokenPackages(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("id"))
	if err != nil || userId <= 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid user id"})
		return
	}
	includeInactive := c.Query("include_inactive") != "false"
	rows, err := model.ListModelTokenPackagesByUser(userId, includeInactive)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rows)
}

func AdminGrantUserModelTokenPackage(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("id"))
	if err != nil || userId <= 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid user id"})
		return
	}
	var req modelTokenPackageGrantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if _, err := model.GetUserById(userId, false); err != nil {
		common.ApiError(c, err)
		return
	}
	inputRatio := 1.0
	outputRatio := 1.0
	cacheRatio := 1.0
	if req.InputRatio != nil {
		inputRatio = *req.InputRatio
	}
	if req.OutputRatio != nil {
		outputRatio = *req.OutputRatio
	}
	if req.CacheRatio != nil {
		cacheRatio = *req.CacheRatio
	}
	pkg, err := model.CreateModelTokenPackage(model.ModelTokenPackageCreateInput{
		UserId:      userId,
		Name:        strings.TrimSpace(req.Name),
		Models:      req.Models,
		TotalTokens: req.TotalTokens,
		InputRatio:  inputRatio,
		OutputRatio: outputRatio,
		CacheRatio:  cacheRatio,
		Priority:    req.Priority,
		ExpiredAt:   req.ExpiredAt,
		Source:      model.ModelTokenPackageSourceAdminGrant,
		CreatedBy:   c.GetInt("id"),
		Remark:      req.Remark,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.RecordLogWithAdminInfo(c.GetInt("id"), model.LogTypeManage,
		"grant model token package user="+strconv.Itoa(userId)+" package="+strconv.Itoa(pkg.Id)+" tokens="+strconv.FormatInt(pkg.TotalTokens, 10),
		map[string]interface{}{
			"target_user_id": userId,
			"package_id":     pkg.Id,
			"total_tokens":   pkg.TotalTokens,
			"models":         pkg.Models,
			"input_ratio":    pkg.InputRatio,
			"output_ratio":   pkg.OutputRatio,
			"cache_ratio":    pkg.CacheRatio,
		})
	common.ApiSuccess(c, pkg)
}

func AdminAdjustUserModelTokenPackage(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("id"))
	if err != nil || userId <= 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid user id"})
		return
	}
	packageId, err := strconv.Atoi(c.Param("pkg_id"))
	if err != nil || packageId <= 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid package id"})
		return
	}
	var req modelTokenPackageAdjustRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	pkg, err := model.AdjustModelTokenPackage(packageId, userId, req.Delta, req.Reason, c.GetInt("id"))
	if err != nil {
		if errors.Is(err, model.ErrModelTokenPackageNotFound) || errors.Is(err, model.ErrModelTokenPackageInsufficient) {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
		common.ApiError(c, err)
		return
	}
	model.RecordLogWithAdminInfo(c.GetInt("id"), model.LogTypeManage,
		"adjust model token package user="+strconv.Itoa(userId)+" package="+strconv.Itoa(packageId)+" delta="+strconv.FormatInt(req.Delta, 10),
		map[string]interface{}{
			"target_user_id": userId,
			"package_id":     packageId,
			"delta":          req.Delta,
			"remaining":      pkg.RemainingTokens,
		})
	common.ApiSuccess(c, pkg)
}

func AdminDisableUserModelTokenPackage(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("id"))
	if err != nil || userId <= 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid user id"})
		return
	}
	packageId, err := strconv.Atoi(c.Param("pkg_id"))
	if err != nil || packageId <= 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid package id"})
		return
	}
	pkg, err := model.DisableModelTokenPackage(packageId, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.RecordLogWithAdminInfo(c.GetInt("id"), model.LogTypeManage,
		"disable model token package user="+strconv.Itoa(userId)+" package="+strconv.Itoa(packageId),
		map[string]interface{}{
			"target_user_id": userId,
			"package_id":     packageId,
		})
	common.ApiSuccess(c, pkg)
}
