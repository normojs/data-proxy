package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/bytedance/gopkg/util/gopool"
)

const (
	exchangeRateProvider           = "frankfurter"
	exchangeRateAutoUpdateTick     = 12 * time.Hour
	exchangeRateHTTPRequestTimeout = 8 * time.Second
)

var (
	exchangeRateCurrencyRegex = regexp.MustCompile(`^[A-Z]{3}$`)
	exchangeRateTaskOnce      sync.Once
	exchangeRateTaskRunning   atomic.Bool
)

type ExchangeRateResult struct {
	CurrencyCode string  `json:"currency_code"`
	Rate         float64 `json:"rate"`
	Provider     string  `json:"provider"`
	UpdatedAt    int64   `json:"updated_at"`
	OptionKey    string  `json:"option_key"`
}

type frankfurterLatestResponse struct {
	Base  string             `json:"base"`
	Rates map[string]float64 `json:"rates"`
}

func normalizeExchangeRateCurrency(code string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(code))
	if !exchangeRateCurrencyRegex.MatchString(normalized) {
		return "", errors.New("currency_code must be a three-letter ISO currency code")
	}
	return normalized, nil
}

func FetchUSDExchangeRate(ctx context.Context, currencyCode string) (ExchangeRateResult, error) {
	code, err := normalizeExchangeRateCurrency(currencyCode)
	if err != nil {
		return ExchangeRateResult{}, err
	}
	if code == "USD" {
		return ExchangeRateResult{
			CurrencyCode: code,
			Rate:         1,
			Provider:     exchangeRateProvider,
			UpdatedAt:    common.GetTimestamp(),
		}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, exchangeRateHTTPRequestTimeout)
	defer cancel()

	url := fmt.Sprintf("https://api.frankfurter.app/latest?from=USD&to=%s", code)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ExchangeRateResult{}, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ExchangeRateResult{}, fmt.Errorf("exchange rate provider unavailable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ExchangeRateResult{}, fmt.Errorf("exchange rate provider returned status %d", resp.StatusCode)
	}

	var payload frankfurterLatestResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return ExchangeRateResult{}, fmt.Errorf("failed to decode exchange rate response: %w", err)
	}
	rate := payload.Rates[code]
	if rate <= 0 {
		return ExchangeRateResult{}, fmt.Errorf("exchange rate for %s is not available", code)
	}
	return ExchangeRateResult{
		CurrencyCode: code,
		Rate:         rate,
		Provider:     exchangeRateProvider,
		UpdatedAt:    common.GetTimestamp(),
	}, nil
}

func resolveExchangeRateTarget(currencyCode string) (string, string, error) {
	if strings.TrimSpace(currencyCode) != "" {
		code, err := normalizeExchangeRateCurrency(currencyCode)
		return code, "USDExchangeRate", err
	}

	general := operation_setting.GetGeneralSetting()
	switch general.QuotaDisplayType {
	case operation_setting.QuotaDisplayTypeCNY:
		return "CNY", "USDExchangeRate", nil
	case operation_setting.QuotaDisplayTypeCustom:
		code, err := normalizeExchangeRateCurrency(general.CustomCurrencyCode)
		return code, "general_setting.custom_currency_exchange_rate", err
	default:
		return "", "", errors.New("current display mode does not have a fetchable exchange rate; choose CNY or Custom Currency, or provide currency_code")
	}
}

func UpdateUSDExchangeRateFromProvider(ctx context.Context, currencyCode string) (ExchangeRateResult, error) {
	code, optionKey, err := resolveExchangeRateTarget(currencyCode)
	if err != nil {
		return ExchangeRateResult{}, err
	}
	result, err := FetchUSDExchangeRate(ctx, code)
	if err != nil {
		return ExchangeRateResult{}, err
	}
	result.OptionKey = optionKey

	values := map[string]string{
		optionKey:                                           fmt.Sprintf("%.6f", result.Rate),
		"general_setting.exchange_rate_provider":            result.Provider,
		"general_setting.exchange_rate_auto_updated_at":     fmt.Sprintf("%d", result.UpdatedAt),
		"general_setting.exchange_rate_auto_update_enabled": fmt.Sprintf("%t", operation_setting.GetGeneralSetting().ExchangeRateAutoUpdateEnabled),
	}
	if optionKey == "general_setting.custom_currency_exchange_rate" {
		values["general_setting.custom_currency_code"] = result.CurrencyCode
	}
	if err := model.UpdateOptionsBulk(values); err != nil {
		return ExchangeRateResult{}, err
	}
	return result, nil
}

func StartExchangeRateAutoUpdateTask() {
	exchangeRateTaskOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("exchange rate auto-update task started: tick=%s", exchangeRateAutoUpdateTick))
			runExchangeRateAutoUpdateOnce()
			ticker := time.NewTicker(exchangeRateAutoUpdateTick)
			defer ticker.Stop()
			for range ticker.C {
				runExchangeRateAutoUpdateOnce()
			}
		})
	})
}

func runExchangeRateAutoUpdateOnce() {
	if !operation_setting.GetGeneralSetting().ExchangeRateAutoUpdateEnabled {
		return
	}
	if !exchangeRateTaskRunning.CompareAndSwap(false, true) {
		return
	}
	defer exchangeRateTaskRunning.Store(false)

	result, err := UpdateUSDExchangeRateFromProvider(context.Background(), "")
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("exchange rate auto-update failed: %v", err))
		return
	}
	logger.LogInfo(context.Background(), fmt.Sprintf("exchange rate updated: currency=%s rate=%.6f provider=%s option=%s", result.CurrencyCode, result.Rate, result.Provider, result.OptionKey))
}
