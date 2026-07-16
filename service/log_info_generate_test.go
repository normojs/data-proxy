package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/stretchr/testify/assert"
)

func TestAppendBillingInfoWallet(t *testing.T) {
	other := map[string]interface{}{}
	appendBillingInfo(&relaycommon.RelayInfo{BillingSource: BillingSourceWallet}, other)
	assert.Equal(t, BillingSourceWallet, other["funding_source"])
	assert.Equal(t, BillingSourceWallet, other["billing_source"])
	ApplyWalletFundingAmount(other, 123)
	assert.Equal(t, 123, other["wallet_quota_deducted"])
}

func TestAppendBillingInfoSubscription(t *testing.T) {
	other := map[string]interface{}{}
	info := &relaycommon.RelayInfo{
		BillingSource:                         BillingSourceSubscription,
		SubscriptionId:                        9,
		SubscriptionPlanId:                    3,
		SubscriptionPlanTitle:                 "Pro",
		SubscriptionPreConsumed:               100,
		SubscriptionPostDelta:                 -20,
		SubscriptionAmountTotal:               1000,
		SubscriptionAmountUsedAfterPreConsume: 100,
	}
	appendBillingInfo(info, other)
	assert.Equal(t, BillingSourceSubscription, other["funding_source"])
	assert.Equal(t, 0, other["wallet_quota_deducted"])
	assert.EqualValues(t, 80, other["subscription_consumed"])
	assert.EqualValues(t, 920, other["subscription_remain"])
	ApplyWalletFundingAmount(other, 999)
	assert.Equal(t, 0, other["wallet_quota_deducted"])
}

func TestAppendBillingInfoPackage(t *testing.T) {
	other := map[string]interface{}{}
	info := &relaycommon.RelayInfo{
		BillingSource: BillingSourceModelTokenPackage,
		Billing: &ModelTokenPackageBillingSession{
			pkg: &model.ModelTokenPackage{
				Id:              7,
				RemainingTokens: 4000,
				InputRatio:      1,
				OutputRatio:     1,
				CacheRatio:      1,
			},
			LastConsume: 120,
		},
	}
	appendBillingInfo(info, other)
	assert.Equal(t, BillingSourceModelTokenPackage, other["funding_source"])
	assert.Equal(t, BillingSourceModelTokenPackage, other["billing_source"])
	assert.Equal(t, 7, other["package_id"])
	assert.EqualValues(t, 120, other["package_consume"])
	assert.EqualValues(t, 4000, other["package_remaining"])
	assert.Equal(t, 0, other["wallet_quota_deducted"])
	ApplyWalletFundingAmount(other, 500)
	assert.Equal(t, 0, other["wallet_quota_deducted"])
}
