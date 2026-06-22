package controller

import "github.com/QuantumNous/new-api/setting/operation_setting"

const defaultTopupDiscount = 1.0

func getTopupDiscountForAmount(amount int64) float64 {
	if amount <= 0 {
		return defaultTopupDiscount
	}

	var (
		matchedThreshold int64
		discount         = defaultTopupDiscount
	)

	for threshold, rate := range operation_setting.GetPaymentSetting().AmountDiscount {
		minAmount := int64(threshold)
		if minAmount <= 0 || minAmount > amount || rate <= 0 {
			continue
		}
		if minAmount >= matchedThreshold {
			matchedThreshold = minAmount
			discount = rate
		}
	}

	return discount
}
