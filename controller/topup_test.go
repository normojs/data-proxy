package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func TestGetBasePayMethodsForTopUpHidesLegacyEpayMethodsWithoutEpayConfig(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	originalPayAddress := operation_setting.PayAddress
	originalEpayID := operation_setting.EpayId
	originalEpayKey := operation_setting.EpayKey
	originalPayMethods := operation_setting.PayMethods
	t.Cleanup(func() {
		operation_setting.PayAddress = originalPayAddress
		operation_setting.EpayId = originalEpayID
		operation_setting.EpayKey = originalEpayKey
		operation_setting.PayMethods = originalPayMethods
	})

	operation_setting.PayMethods = []map[string]string{
		{"name": "微信", "type": "wxpay", "color": "#07C160"},
	}
	operation_setting.PayAddress = ""
	operation_setting.EpayId = ""
	operation_setting.EpayKey = ""

	require.Empty(t, getBasePayMethodsForTopUp(true))

	operation_setting.PayAddress = "https://pay.example.com"
	operation_setting.EpayId = "epay_id"
	operation_setting.EpayKey = "epay_key"

	methods := getBasePayMethodsForTopUp(true)
	require.Len(t, methods, 1)
	require.Equal(t, "wxpay", methods[0]["type"])

	methods[0]["type"] = "changed"
	require.Equal(t, "wxpay", operation_setting.PayMethods[0]["type"])
}

func TestBuildWechatPayTopUpMethodUsesConfiguredWxpayDisplayName(t *testing.T) {
	originalPayMethods := operation_setting.PayMethods
	t.Cleanup(func() {
		operation_setting.PayMethods = originalPayMethods
	})

	operation_setting.PayMethods = []map[string]string{
		{
			"name":  "微信",
			"type":  "wxpay",
			"color": "#07C160",
			"icon":  "https://example.com/wechat.svg",
		},
	}

	method := buildWechatPayTopUpMethod(3)

	require.Equal(t, "微信", method["name"])
	require.Equal(t, model.PaymentMethodWechatPay, method["type"])
	require.Equal(t, "#07C160", method["color"])
	require.Equal(t, "https://example.com/wechat.svg", method["icon"])
	require.Equal(t, "3", method["min_topup"])
}
