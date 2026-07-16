package service

import (
	"fmt"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// UserFacingBillingError builds a stable-code 403 with bilingual human message.
// Detailed remain/need amounts should stay in server logs, not the public message.
func UserFacingBillingError(c *gin.Context, code types.ErrorCode) *types.NewAPIError {
	key := billingUserFacingMessageKey(code)
	msg := translateBillingMessage(c, key, code)
	return types.NewErrorWithStatusCode(
		fmt.Errorf("%s", msg),
		code,
		http.StatusForbidden,
		types.ErrOptionWithSkipRetry(),
		types.ErrOptionWithNoRecordErrorLog(),
	)
}

func translateBillingMessage(c *gin.Context, key string, code types.ErrorCode) string {
	if c != nil && common.TranslateMessage != nil {
		msg := common.TranslateMessage(c, key)
		if msg != "" && msg != key {
			return msg
		}
	}
	return billingUserFacingFallback(code)
}

func billingUserFacingMessageKey(code types.ErrorCode) string {
	switch code {
	case types.ErrorCodeInsufficientModelTokenPackage:
		return i18n.MsgBillingInsufficientModelTokenPackage
	case types.ErrorCodePreConsumeTokenQuotaFailed:
		return i18n.MsgBillingPreConsumeTokenQuotaFailed
	case types.ErrorCodeInsufficientUserQuota:
		return i18n.MsgBillingInsufficientUserQuota
	default:
		return i18n.MsgBillingInsufficientUserQuota
	}
}

// UserFacingSubscriptionQuotaError uses the same stable code as wallet for
// back-compat, but a clearer subscription-oriented message.
func UserFacingSubscriptionQuotaError(c *gin.Context) *types.NewAPIError {
	const fallback = "Subscription quota is insufficient or no active subscription is available. Please renew/purchase a plan, or switch to wallet billing."
	msg := fallback
	key := i18n.MsgBillingInsufficientSubscriptionQuota
	if c != nil && common.TranslateMessage != nil {
		if translated := common.TranslateMessage(c, key); translated != "" && translated != key {
			msg = translated
		}
	}
	return types.NewErrorWithStatusCode(
		fmt.Errorf("%s", msg),
		types.ErrorCodeInsufficientUserQuota,
		http.StatusForbidden,
		types.ErrOptionWithSkipRetry(),
		types.ErrOptionWithNoRecordErrorLog(),
	)
}

func billingUserFacingFallback(code types.ErrorCode) string {
	switch code {
	case types.ErrorCodeInsufficientModelTokenPackage:
		return "Your model token package for this model is insufficient. Please purchase/redeem a package, switch model, or use wallet billing when no package applies."
	case types.ErrorCodePreConsumeTokenQuotaFailed:
		return "This API key has reached its quota limit. Please raise the key limit in /keys, create a new key, or use an unlimited key."
	default:
		return "Wallet balance is insufficient for this request. Please top up your wallet at /wallet, or switch billing preference / enable a subscription."
	}
}
