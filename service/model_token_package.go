package service

import (
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const BillingSourceModelTokenPackage = "model_token_package"

// ModelTokenPackageBillingSession bills usage against a model token package (LLM tokens).
// Settle receives actualQuota in money-points from wallet path callers; for package path
// Settle uses stored usage token counts instead of actualQuota.
type ModelTokenPackageBillingSession struct {
	relayInfo *relaycommon.RelayInfo
	pkg       *model.ModelTokenPackage
	settled   bool
	refunded  bool
	mu        sync.Mutex

	// filled at settle time for logging
	LastConsume int64
}

func (s *ModelTokenPackageBillingSession) Settle(actualQuota int) error {
	// Money-quota Settle is a no-op for package funding.
	// Real package debit happens in SettleWithUsage / SettleModelTokenPackageIfNeeded.
	return nil
}

func (s *ModelTokenPackageBillingSession) SettleWithUsage(usage *dto.Usage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.settled {
		return nil
	}
	if s.pkg == nil {
		s.settled = true
		return nil
	}
	prompt, completion, cache := ExtractUsageTokenParts(usage)
	consume := ComputeModelTokenPackageConsume(prompt, completion, cache, s.pkg.InputRatio, s.pkg.OutputRatio, s.pkg.CacheRatio)
	if consume > 0 {
		updated, err := model.ConsumeModelTokenPackage(model.ModelTokenPackageConsumeInput{
			PackageId:        s.pkg.Id,
			UserId:           s.relayInfo.UserId,
			RequestId:        s.relayInfo.RequestId,
			Model:            s.relayInfo.OriginModelName,
			PromptTokens:     prompt,
			CompletionTokens: completion,
			CacheTokens:      cache,
			ConsumeTokens:    consume,
		})
		if err != nil {
			return err
		}
		s.pkg = updated
		s.LastConsume = consume
	}
	s.settled = true
	return nil
}

func (s *ModelTokenPackageBillingSession) Refund(c *gin.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// P0: no pre-reservation; nothing to refund.
	s.refunded = true
}

func (s *ModelTokenPackageBillingSession) NeedsRefund() bool {
	return false
}

func (s *ModelTokenPackageBillingSession) GetPreConsumedQuota() int {
	return 0
}

func (s *ModelTokenPackageBillingSession) Reserve(targetQuota int) error {
	return nil
}

func (s *ModelTokenPackageBillingSession) Package() *model.ModelTokenPackage {
	return s.pkg
}

// ComputeModelTokenPackageConsume applies input/output/cache ratios and ceils to int64 tokens.
func ComputeModelTokenPackageConsume(promptTokens, completionTokens, cacheTokens int, inputRatio, outputRatio, cacheRatio float64) int64 {
	if promptTokens < 0 {
		promptTokens = 0
	}
	if completionTokens < 0 {
		completionTokens = 0
	}
	if cacheTokens < 0 {
		cacheTokens = 0
	}
	inputRatio = model.ResolveModelTokenPackageRatio(inputRatio)
	outputRatio = model.ResolveModelTokenPackageRatio(outputRatio)
	cacheRatio = model.ResolveModelTokenPackageRatio(cacheRatio)
	raw := float64(promptTokens)*inputRatio + float64(completionTokens)*outputRatio + float64(cacheTokens)*cacheRatio
	if raw <= 0 {
		return 0
	}
	return int64(math.Ceil(raw - 1e-9))
}

// ExtractUsageTokenParts returns prompt, completion, and cache token counts from usage.
// cache = cached read + cache creation (all cache-related tokens counted for package metering).
func ExtractUsageTokenParts(usage *dto.Usage) (prompt, completion, cache int) {
	if usage == nil {
		return 0, 0, 0
	}
	prompt = usage.PromptTokens
	completion = usage.CompletionTokens
	cache = usage.PromptTokensDetails.CachedTokens + usage.PromptTokensDetails.CachedCreationTokens
	if cache == 0 && usage.PromptCacheHitTokens > 0 {
		cache = usage.PromptCacheHitTokens
	}
	// Claude-style fields if present on usage
	if usage.ClaudeCacheCreation5mTokens > 0 || usage.ClaudeCacheCreation1hTokens > 0 {
		cache += usage.ClaudeCacheCreation5mTokens + usage.ClaudeCacheCreation1hTokens
	}
	return prompt, completion, cache
}

// ResolveModelTokenPackage finds the best active package covering model for user.
func ResolveModelTokenPackage(userId int, modelName string) (*model.ModelTokenPackage, error) {
	if userId <= 0 {
		return nil, nil
	}
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return nil, nil
	}
	matchName := ratio_setting.FormatMatchingModelName(modelName)
	now := common.GetTimestamp()
	packages, err := model.ListActiveModelTokenPackagesForUser(userId, now)
	if err != nil {
		return nil, err
	}
	for i := range packages {
		pkg := &packages[i]
		if model.PackageCoversModel(*pkg, modelName) || model.PackageCoversModel(*pkg, matchName) {
			// also try matching each package model through FormatMatchingModelName
			return pkg, nil
		}
		for _, allowed := range pkg.Models {
			if ratio_setting.FormatMatchingModelName(allowed) == matchName || strings.EqualFold(allowed, modelName) {
				return pkg, nil
			}
		}
	}
	return nil, nil
}

// TryAttachModelTokenPackageBilling selects package funding when available.
// Returns true when package billing is attached (wallet pre-consume skipped).
func TryAttachModelTokenPackageBilling(c *gin.Context, relayInfo *relaycommon.RelayInfo) (bool, *types.NewAPIError) {
	if relayInfo == nil || relayInfo.UserId <= 0 {
		return false, nil
	}
	modelName := strings.TrimSpace(relayInfo.OriginModelName)
	if modelName == "" {
		return false, nil
	}
	pkg, err := ResolveModelTokenPackage(relayInfo.UserId, modelName)
	if err != nil {
		return false, types.NewError(err, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
	}
	if pkg == nil {
		return false, nil
	}
	if pkg.RemainingTokens <= 0 {
		if c != nil {
			logger.LogInfo(c, fmt.Sprintf("model token package exhausted for model %s, package_id=%d remaining=0", modelName, pkg.Id))
		}
		return false, UserFacingBillingError(c, types.ErrorCodeInsufficientModelTokenPackage)
	}
	session := &ModelTokenPackageBillingSession{
		relayInfo: relayInfo,
		pkg:       pkg,
	}
	relayInfo.Billing = session
	relayInfo.BillingSource = BillingSourceModelTokenPackage
	relayInfo.FinalPreConsumedQuota = 0
	if c != nil {
		logger.LogInfo(c, fmt.Sprintf("using model token package id=%d remaining=%d models=%v for model=%s",
			pkg.Id, pkg.RemainingTokens, pkg.Models, modelName))
	}
	return true, nil
}

// SettleModelTokenPackageIfNeeded settles package billing using usage when active.
// For wallet sessions this is a no-op.
func SettleModelTokenPackageIfNeeded(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *dto.Usage) error {
	if relayInfo == nil || relayInfo.Billing == nil {
		return nil
	}
	session, ok := relayInfo.Billing.(*ModelTokenPackageBillingSession)
	if !ok {
		return nil
	}
	if err := session.SettleWithUsage(usage); err != nil {
		if err == model.ErrModelTokenPackageInsufficient {
			if ctx != nil {
				logger.LogInfo(ctx, fmt.Sprintf("model token package insufficient for model %s", relayInfo.OriginModelName))
			}
			return UserFacingBillingError(ctx, types.ErrorCodeInsufficientModelTokenPackage)
		}
		return err
	}
	if session.LastConsume > 0 && ctx != nil {
		logger.LogInfo(ctx, fmt.Sprintf("model token package settled: package_id=%d consume=%d remaining=%d",
			session.pkg.Id, session.LastConsume, session.pkg.RemainingTokens))
	}
	return nil
}

func IsModelTokenPackageBilling(relayInfo *relaycommon.RelayInfo) bool {
	if relayInfo == nil {
		return false
	}
	if relayInfo.BillingSource == BillingSourceModelTokenPackage {
		return true
	}
	_, ok := relayInfo.Billing.(*ModelTokenPackageBillingSession)
	return ok
}

func ModelTokenPackageBillingOtherInfo(relayInfo *relaycommon.RelayInfo) map[string]any {
	if relayInfo == nil {
		return nil
	}
	session, ok := relayInfo.Billing.(*ModelTokenPackageBillingSession)
	if !ok || session.pkg == nil {
		return nil
	}
	return map[string]any{
		"funding_source":        BillingSourceModelTokenPackage,
		"billing_source":        BillingSourceModelTokenPackage,
		"package_id":            session.pkg.Id,
		"package_consume":       session.LastConsume,
		"package_remaining":     session.pkg.RemainingTokens,
		"input_ratio":           session.pkg.InputRatio,
		"output_ratio":          session.pkg.OutputRatio,
		"cache_ratio":           session.pkg.CacheRatio,
		"wallet_quota_deducted": 0,
	}
}
