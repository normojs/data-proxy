package setting

var (
	WechatPayEnabled          bool
	WechatPayAppID            string
	WechatPayMchID            string
	WechatPayAPIv3Key         string
	WechatPayMerchantSerialNo string
	WechatPayPrivateKey       string
	WechatPayPrivateKeyPath   string
	WechatPayNotifyUrl        string
	WechatPayProductName      string = "Data Proxy Topup"
	WechatPayMinTopUp         int    = 1
)
