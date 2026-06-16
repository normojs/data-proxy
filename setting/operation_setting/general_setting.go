package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

// 额度展示类型
const (
	QuotaDisplayTypeUSD    = "USD"
	QuotaDisplayTypeCNY    = "CNY"
	QuotaDisplayTypeTokens = "TOKENS"
	QuotaDisplayTypeCustom = "CUSTOM"
)

type GeneralSetting struct {
	DocsLink            string `json:"docs_link"`
	PingIntervalEnabled bool   `json:"ping_interval_enabled"`
	PingIntervalSeconds int    `json:"ping_interval_seconds"`
	// 当前站点额度展示类型：USD / CNY / TOKENS
	QuotaDisplayType string `json:"quota_display_type"`
	// 自定义货币符号，用于 CUSTOM 展示类型
	CustomCurrencySymbol string `json:"custom_currency_symbol"`
	// 自定义货币 ISO 4217 代码，用于自动获取汇率
	CustomCurrencyCode string `json:"custom_currency_code"`
	// 自定义货币与美元汇率（1 USD = X Custom）
	CustomCurrencyExchangeRate float64 `json:"custom_currency_exchange_rate"`
	// 是否使用公共汇率源自动更新展示汇率
	ExchangeRateAutoUpdateEnabled bool `json:"exchange_rate_auto_update_enabled"`
	// 自动刷新汇率间隔（分钟）
	ExchangeRateAutoUpdateIntervalMinutes int `json:"exchange_rate_auto_update_interval_minutes"`
	// 最近一次自动或手动汇率更新时间
	ExchangeRateAutoUpdatedAt int64 `json:"exchange_rate_auto_updated_at"`
	// 最近一次汇率来源
	ExchangeRateProvider string `json:"exchange_rate_provider"`
}

// 默认配置
var generalSetting = GeneralSetting{
	DocsLink:                              "https://docs.newapi.pro",
	PingIntervalEnabled:                   false,
	PingIntervalSeconds:                   60,
	QuotaDisplayType:                      QuotaDisplayTypeUSD,
	CustomCurrencySymbol:                  "¤",
	CustomCurrencyCode:                    "CNY",
	CustomCurrencyExchangeRate:            1.0,
	ExchangeRateAutoUpdateIntervalMinutes: 720,
	ExchangeRateProvider:                  "frankfurter",
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("general_setting", &generalSetting)
}

func GetGeneralSetting() *GeneralSetting {
	return &generalSetting
}

// IsCurrencyDisplay 是否以货币形式展示（美元或人民币）
func IsCurrencyDisplay() bool {
	return generalSetting.QuotaDisplayType != QuotaDisplayTypeTokens
}

// IsCNYDisplay 是否以人民币展示
func IsCNYDisplay() bool {
	return generalSetting.QuotaDisplayType == QuotaDisplayTypeCNY
}

// GetQuotaDisplayType 返回额度展示类型
func GetQuotaDisplayType() string {
	return generalSetting.QuotaDisplayType
}

// GetCurrencySymbol 返回当前展示类型对应符号
func GetCurrencySymbol() string {
	switch generalSetting.QuotaDisplayType {
	case QuotaDisplayTypeUSD:
		return "$"
	case QuotaDisplayTypeCNY:
		return "¥"
	case QuotaDisplayTypeCustom:
		if generalSetting.CustomCurrencySymbol != "" {
			return generalSetting.CustomCurrencySymbol
		}
		return "¤"
	default:
		return ""
	}
}

// GetUsdToCurrencyRate 返回 1 USD = X <currency> 的 X（TOKENS 不适用）
func GetUsdToCurrencyRate(usdToCny float64) float64 {
	switch generalSetting.QuotaDisplayType {
	case QuotaDisplayTypeUSD:
		return 1
	case QuotaDisplayTypeCNY:
		return usdToCny
	case QuotaDisplayTypeCustom:
		if generalSetting.CustomCurrencyExchangeRate > 0 {
			return generalSetting.CustomCurrencyExchangeRate
		}
		return 1
	default:
		return 1
	}
}
