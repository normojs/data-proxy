package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type buildTrainingDatasetRequest struct {
	Name                  string `json:"name"`
	Version               string `json:"version"`
	StartTimestamp        int64  `json:"start_timestamp"`
	EndTimestamp          int64  `json:"end_timestamp"`
	Limit                 int    `json:"limit"`
	MaxDecodedBundleBytes int64  `json:"max_decoded_bundle_bytes"`
	IncludeErrored        bool   `json:"include_errored"`
	OutputStorageKey      string `json:"output_storage_key"`
}

type reviewTrainingSampleRequest struct {
	Comment string `json:"comment"`
}

func ListTrainingDatasets(c *gin.Context) {
	if model.DB == nil {
		common.ApiErrorMsg(c, "database is not initialized")
		return
	}
	pageInfo := common.GetPageQuery(c)
	tx := model.DB.Model(&model.TrainingDatasetVersion{})
	if name := strings.TrimSpace(c.Query("name")); name != "" {
		tx = tx.Where("name = ?", name)
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		tx = tx.Where("status = ?", status)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var datasets []model.TrainingDatasetVersion
	if err := tx.Order("id desc").Offset(pageInfo.GetStartIdx()).Limit(pageInfo.GetPageSize()).Find(&datasets).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(datasets)
	common.ApiSuccess(c, pageInfo)
}

func BuildTrainingDataset(c *gin.Context) {
	var req buildTrainingDatasetRequest
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	result, err := service.BuildTrainingCorpusDataset(c.Request.Context(), service.TrainingCorpusBuildOptions{
		Name:                  req.Name,
		Version:               req.Version,
		StartTimestamp:        req.StartTimestamp,
		EndTimestamp:          req.EndTimestamp,
		Limit:                 req.Limit,
		MaxDecodedBundleBytes: req.MaxDecodedBundleBytes,
		IncludeErrored:        req.IncludeErrored,
		OutputStorageKey:      req.OutputStorageKey,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"dataset": result.Dataset,
		"object":  result.Object,
		"samples": len(result.Samples),
		"skipped": result.Skipped,
		"errors":  result.Errors,
	})
}

func ListTrainingSamples(c *gin.Context) {
	if model.DB == nil {
		common.ApiErrorMsg(c, "database is not initialized")
		return
	}
	pageInfo := common.GetPageQuery(c)
	tx := model.DB.Model(&model.TrainingSample{})
	if datasetVersionId, _ := strconv.ParseInt(c.Query("dataset_version_id"), 10, 64); datasetVersionId > 0 {
		tx = tx.Where("dataset_version_id = ?", datasetVersionId)
	}
	if requestId := strings.TrimSpace(c.Query("request_id")); requestId != "" {
		tx = tx.Where("request_id = ?", requestId)
	}
	if modelName := strings.TrimSpace(c.Query("model_name")); modelName != "" {
		tx = tx.Where("model_name = ?", modelName)
	}
	if minQuality, _ := strconv.Atoi(c.Query("min_quality_score")); minQuality > 0 {
		tx = tx.Where("quality_score >= ?", minQuality)
	}
	if reviewStatus := strings.TrimSpace(c.Query("review_status")); reviewStatus != "" {
		tx = tx.Where("review_status = ?", reviewStatus)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var samples []model.TrainingSample
	if err := tx.Order("id desc").Offset(pageInfo.GetStartIdx()).Limit(pageInfo.GetPageSize()).Find(&samples).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(samples)
	common.ApiSuccess(c, pageInfo)
}

func ApproveTrainingSample(c *gin.Context) {
	reviewTrainingSample(c, model.TrainingSampleReviewStatusApproved)
}

func RejectTrainingSample(c *gin.Context) {
	reviewTrainingSample(c, model.TrainingSampleReviewStatusRejected)
}

func GetTrainingSamplePreview(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid sample id")
		return
	}
	preview, err := service.LoadTrainingSamplePreview(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrTrainingSamplePreviewNotFound) {
			common.ApiErrorMsg(c, "training sample preview not found")
			return
		}
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, preview)
}

func DownloadTrainingDatasetExport(c *gin.Context) {
	if model.DB == nil {
		common.ApiErrorMsg(c, "database is not initialized")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid dataset id")
		return
	}
	var dataset model.TrainingDatasetVersion
	if err := model.DB.First(&dataset, id).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if dataset.Status != model.TrainingDatasetStatusCompleted || strings.TrimSpace(dataset.StorageKey) == "" {
		common.ApiErrorMsg(c, "training dataset export is not available")
		return
	}
	export, err := service.BuildApprovedTrainingDatasetExport(c.Request.Context(), dataset)
	if err != nil {
		if errors.Is(err, service.ErrTrainingDatasetNoApprovedSamples) {
			common.ApiErrorMsg(c, "training dataset has no approved samples")
			return
		}
		common.ApiError(c, err)
		return
	}
	filename := fmt.Sprintf("data-proxy-training-%s-%s-approved.jsonl.zst", sanitizeDiagnosticBundleFileName(dataset.Name), sanitizeDiagnosticBundleFileName(dataset.Version))
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Header("X-Data-Proxy-Training-Dataset-Id", strconv.FormatInt(dataset.Id, 10))
	c.Header("X-Data-Proxy-Training-Dataset-SHA256", export.SHA256)
	c.Header("X-Data-Proxy-Training-Dataset-Source-SHA256", dataset.SHA256)
	c.Header("X-Data-Proxy-Training-Dataset-Source-Samples", strconv.FormatInt(export.SourceSamples, 10))
	c.Header("X-Data-Proxy-Training-Dataset-Approved-Samples", strconv.FormatInt(export.ApprovedSamples, 10))
	c.Header("X-Data-Proxy-Training-Dataset-Exported-Samples", strconv.FormatInt(export.ExportedSamples, 10))
	c.Data(http.StatusOK, "application/zstd", export.Body)
}

func reviewTrainingSample(c *gin.Context, status string) {
	if model.DB == nil {
		common.ApiErrorMsg(c, "database is not initialized")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid sample id")
		return
	}
	var req reviewTrainingSampleRequest
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	var sample model.TrainingSample
	if err := model.DB.First(&sample, id).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	now := common.GetTimestamp()
	comment := strings.TrimSpace(req.Comment)
	if len(comment) > 1024 {
		comment = comment[:1024]
	}
	updates := map[string]interface{}{
		"review_status":  status,
		"review_comment": comment,
		"reviewed_by":    c.GetInt("id"),
		"reviewed_at":    now,
		"updated_at":     now,
	}
	if err := model.DB.Model(&model.TrainingSample{}).Where("id = ?", sample.Id).Updates(updates).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	sample.ReviewStatus = status
	sample.ReviewComment = comment
	sample.ReviewedBy = c.GetInt("id")
	sample.ReviewedAt = now
	common.ApiSuccess(c, gin.H{"sample": sample})
}
