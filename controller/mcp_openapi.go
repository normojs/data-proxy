package controller

import (
	"errors"
	"mime"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func PreviewMCPOpenAPI(c *gin.Context) {
	var req dto.MCPOpenAPIPreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	item, err := service.PreviewMCPOpenAPIForAdmin(c.Request.Context(), req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func ImportMCPOpenAPI(c *gin.Context) {
	var req dto.MCPOpenAPIImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	item, err := service.ImportMCPOpenAPIForAdmin(c.Request.Context(), req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func DiffMCPOpenAPI(c *gin.Context) {
	var req dto.MCPOpenAPIImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	item, err := service.DiffMCPOpenAPIForAdmin(c.Request.Context(), req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func DisableMCPOpenAPI(c *gin.Context) {
	var req dto.MCPOpenAPILifecycleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	item, err := service.DisableMCPOpenAPIForAdmin(req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func DeleteMCPOpenAPI(c *gin.Context) {
	var req dto.MCPOpenAPILifecycleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	item, err := service.DeleteMCPOpenAPIForAdmin(req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func CleanupMCPOpenAPIBinaryObjects(c *gin.Context) {
	var req dto.MCPOpenAPIBinaryCleanupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	item, err := service.CleanupMCPOpenAPIBinaryObjectsForAdmin(req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func DownloadMCPOpenAPIBinaryObject(c *gin.Context) {
	item, err := service.DownloadMCPOpenAPIBinaryObject(c.Request.Context(), service.MCPOpenAPIBinaryDownloadParams{
		UserId:   c.GetInt("id"),
		IsAdmin:  model.IsAdmin(c.GetInt("id")),
		ObjectId: c.Param("object_id"),
	})
	if err != nil {
		if errors.Is(err, service.ErrMCPOpenAPIBinaryObjectNotFound) {
			c.Status(http.StatusNotFound)
			return
		}
		common.ApiError(c, err)
		return
	}
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{
		"filename": item.Object.Filename,
	}))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, item.Object.ContentType, item.Content)
}
