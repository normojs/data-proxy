package service

import "testing"

func TestNormalizeExchangeRateAutoUpdateIntervalMinutes(t *testing.T) {
	tests := []struct {
		name  string
		input int
		want  int
	}{
		{
			name:  "uses default for unset interval",
			input: 0,
			want:  ExchangeRateAutoUpdateDefaultIntervalMinutes,
		},
		{
			name:  "uses default for negative interval",
			input: -1,
			want:  ExchangeRateAutoUpdateDefaultIntervalMinutes,
		},
		{
			name:  "clamps below minimum interval",
			input: 15,
			want:  ExchangeRateAutoUpdateMinIntervalMinutes,
		},
		{
			name:  "keeps configured interval",
			input: 240,
			want:  240,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeExchangeRateAutoUpdateIntervalMinutes(tt.input); got != tt.want {
				t.Fatalf("NormalizeExchangeRateAutoUpdateIntervalMinutes(%d) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}
