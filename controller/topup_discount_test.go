package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func TestGetTopupDiscountForAmount(t *testing.T) {
	originalDiscounts := make(map[int]float64, len(operation_setting.GetPaymentSetting().AmountDiscount))
	for k, v := range operation_setting.GetPaymentSetting().AmountDiscount {
		originalDiscounts[k] = v
	}

	t.Cleanup(func() {
		operation_setting.GetPaymentSetting().AmountDiscount = originalDiscounts
	})

	operation_setting.GetPaymentSetting().AmountDiscount = map[int]float64{
		1:   0.14,
		10:  0.8,
		100: 0.6,
		200: 0,
	}

	testCases := []struct {
		name     string
		amount   int64
		expected float64
	}{
		{
			name:     "matches lower threshold",
			amount:   5,
			expected: 0.14,
		},
		{
			name:     "uses largest threshold not greater than amount",
			amount:   50,
			expected: 0.8,
		},
		{
			name:     "uses exact threshold when available",
			amount:   100,
			expected: 0.6,
		},
		{
			name:     "ignores non-positive exact threshold and keeps previous valid tier",
			amount:   200,
			expected: 0.6,
		},
		{
			name:     "falls back when amount is below every threshold",
			amount:   0,
			expected: 1.0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.InDelta(t, tc.expected, getTopupDiscountForAmount(tc.amount), 0.000001)
		})
	}
}
