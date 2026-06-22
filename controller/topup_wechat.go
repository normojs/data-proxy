package controller

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	"github.com/wechatpay-apiv3/wechatpay-go/core/downloader"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/native"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
)

const wechatPayOrderTTL = 10 * time.Minute

type WechatPayRequest struct {
	Amount int64 `json:"amount"`
}

func getWechatPayMinTopup() int64 {
	minTopup := setting.WechatPayMinTopUp
	if minTopup <= 0 {
		minTopup = 1
	}
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		minTopup = minTopup * int(common.QuotaPerUnit)
	}
	return int64(minTopup)
}

func normalizeWechatPayTopUpAmount(amount int64) int64 {
	if operation_setting.GetQuotaDisplayType() != operation_setting.QuotaDisplayTypeTokens {
		return amount
	}
	normalized := decimal.NewFromInt(amount).Div(decimal.NewFromFloat(common.QuotaPerUnit)).IntPart()
	if normalized < 1 {
		return 1
	}
	return normalized
}

func wechatPayAmountFen(payMoney float64) int64 {
	return decimal.NewFromFloat(payMoney).Mul(decimal.NewFromInt(100)).Round(0).IntPart()
}

func getWechatPayProductName() string {
	name := strings.TrimSpace(setting.WechatPayProductName)
	if name == "" {
		name = "Data Proxy Topup"
	}
	if len([]rune(name)) > 90 {
		name = string([]rune(name)[:90])
	}
	return name
}

func newWechatPayTradeNo() string {
	return fmt.Sprintf("WXP%d%s", time.Now().UnixMilli(), common.GetRandomString(8))
}

func getWechatPayNotifyUrl() string {
	if notifyUrl := strings.TrimSpace(setting.WechatPayNotifyUrl); notifyUrl != "" {
		return notifyUrl
	}
	return strings.TrimRight(service.GetCallbackAddress(), "/") + "/api/wechat-pay/notify"
}

func loadWechatPayPrivateKey() (*rsa.PrivateKey, error) {
	privateKeyText := strings.TrimSpace(setting.WechatPayPrivateKey)
	if privateKeyText != "" {
		return utils.LoadPrivateKey(privateKeyText)
	}
	privateKeyPath := strings.TrimSpace(setting.WechatPayPrivateKeyPath)
	if privateKeyPath == "" {
		return nil, errors.New("微信支付商户私钥未配置")
	}
	return utils.LoadPrivateKeyWithPath(privateKeyPath)
}

func newWechatPayNativeService(ctx context.Context) (*native.NativeApiService, error) {
	privateKey, err := loadWechatPayPrivateKey()
	if err != nil {
		return nil, err
	}
	client, err := core.NewClient(ctx, option.WithWechatPayAutoAuthCipher(
		strings.TrimSpace(setting.WechatPayMchID),
		strings.TrimSpace(setting.WechatPayMerchantSerialNo),
		privateKey,
		strings.TrimSpace(setting.WechatPayAPIv3Key),
	))
	if err != nil {
		return nil, err
	}
	return &native.NativeApiService{Client: client}, nil
}

func newWechatPayNotifyHandler(ctx context.Context) (*notify.Handler, error) {
	if _, err := newWechatPayNativeService(ctx); err != nil {
		return nil, err
	}
	certificateVisitor := downloader.MgrInstance().GetCertificateVisitor(strings.TrimSpace(setting.WechatPayMchID))
	return notify.NewRSANotifyHandler(
		strings.TrimSpace(setting.WechatPayAPIv3Key),
		verifiers.NewSHA256WithRSAVerifier(certificateVisitor),
	)
}

func RequestWechatPay(c *gin.Context) {
	if !isWechatPayTopUpEnabled() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "微信支付未启用或配置不完整"})
		return
	}

	var req WechatPayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}
	if req.Amount < getWechatPayMinTopup() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("充值数量不能小于 %d", getWechatPayMinTopup())})
		return
	}

	id := c.GetInt("id")
	group, err := model.GetUserGroup(id, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "获取用户分组失败"})
		return
	}
	payMoney := getPayMoney(req.Amount, group)
	payFen := wechatPayAmountFen(payMoney)
	if payFen < 1 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值金额过低"})
		return
	}

	tradeNo := newWechatPayTradeNo()
	expiresAt := time.Now().Add(wechatPayOrderTTL)
	topUp := &model.TopUp{
		UserId:          id,
		Amount:          normalizeWechatPayTopUpAmount(req.Amount),
		Money:           payMoney,
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodWechatPay,
		PaymentProvider: model.PaymentProviderWechatPay,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	if err := topUp.Insert(); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("微信支付 创建充值订单失败 user_id=%d trade_no=%s amount=%d error=%q", id, tradeNo, req.Amount, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}

	svc, err := newWechatPayNativeService(c.Request.Context())
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("微信支付 SDK 初始化失败 user_id=%d trade_no=%s error=%q", id, tradeNo, err.Error()))
		topUp.Status = common.TopUpStatusFailed
		_ = topUp.Update()
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "支付配置错误"})
		return
	}

	resp, _, err := svc.Prepay(c.Request.Context(), native.PrepayRequest{
		Appid:       core.String(strings.TrimSpace(setting.WechatPayAppID)),
		Mchid:       core.String(strings.TrimSpace(setting.WechatPayMchID)),
		Description: core.String(fmt.Sprintf("%s %d", getWechatPayProductName(), req.Amount)),
		OutTradeNo:  core.String(tradeNo),
		TimeExpire:  core.Time(expiresAt),
		Attach:      core.String(fmt.Sprintf("user_id=%d", id)),
		NotifyUrl:   core.String(getWechatPayNotifyUrl()),
		Amount: &native.Amount{
			Currency: core.String("CNY"),
			Total:    core.Int64(payFen),
		},
		SceneInfo: &native.SceneInfo{
			PayerClientIp: core.String(c.ClientIP()),
		},
	})
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("微信支付 创建预支付订单失败 user_id=%d trade_no=%s amount=%d money=%.2f error=%q", id, tradeNo, req.Amount, payMoney, err.Error()))
		topUp.Status = common.TopUpStatusFailed
		_ = topUp.Update()
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}
	if resp == nil || resp.CodeUrl == nil || strings.TrimSpace(*resp.CodeUrl) == "" {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("微信支付 创建预支付订单返回空二维码 user_id=%d trade_no=%s response=%q", id, tradeNo, common.GetJsonString(resp)))
		topUp.Status = common.TopUpStatusFailed
		_ = topUp.Update()
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}

	logger.LogInfo(c.Request.Context(), fmt.Sprintf("微信支付 充值订单创建成功 user_id=%d trade_no=%s amount=%d money=%.2f", id, tradeNo, req.Amount, payMoney))
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data": gin.H{
			"code_url":   *resp.CodeUrl,
			"trade_no":   tradeNo,
			"expires_at": expiresAt.Unix(),
			"money":      payMoney,
		},
	})
}

func wechatPaySuccess(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": "SUCCESS", "message": "成功"})
}

func wechatPayFail(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"code": "FAIL", "message": message})
}

func WechatPayNotify(c *gin.Context) {
	ctx := c.Request.Context()
	if !isWechatPayWebhookEnabled() {
		logger.LogWarn(ctx, fmt.Sprintf("微信支付 webhook 被拒绝 reason=webhook_disabled path=%q client_ip=%s", c.Request.RequestURI, c.ClientIP()))
		wechatPayFail(c, http.StatusForbidden, "微信支付未启用")
		return
	}

	handler, err := newWechatPayNotifyHandler(ctx)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("微信支付 webhook 初始化失败 path=%q client_ip=%s error=%q", c.Request.RequestURI, c.ClientIP(), err.Error()))
		wechatPayFail(c, http.StatusInternalServerError, "支付配置错误")
		return
	}

	transaction := new(payments.Transaction)
	notifyReq, err := handler.ParseNotifyRequest(ctx, c.Request, transaction)
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("微信支付 webhook 验签或解密失败 path=%q client_ip=%s error=%q", c.Request.RequestURI, c.ClientIP(), err.Error()))
		wechatPayFail(c, http.StatusBadRequest, "验签失败")
		return
	}

	tradeNo := ""
	if transaction.OutTradeNo != nil {
		tradeNo = strings.TrimSpace(*transaction.OutTradeNo)
	}
	tradeState := ""
	if transaction.TradeState != nil {
		tradeState = strings.TrimSpace(*transaction.TradeState)
	}
	transactionID := ""
	if transaction.TransactionId != nil {
		transactionID = strings.TrimSpace(*transaction.TransactionId)
	}
	logger.LogInfo(ctx, fmt.Sprintf("微信支付 webhook 验签成功 trade_no=%s trade_state=%s transaction_id=%s client_ip=%s summary=%q", tradeNo, tradeState, transactionID, c.ClientIP(), notifyReq.Summary))

	if tradeNo == "" {
		wechatPayFail(c, http.StatusBadRequest, "缺少商户订单号")
		return
	}
	if tradeState != "SUCCESS" {
		logger.LogInfo(ctx, fmt.Sprintf("微信支付 webhook 忽略非成功事件 trade_no=%s trade_state=%s transaction_id=%s client_ip=%s", tradeNo, tradeState, transactionID, c.ClientIP()))
		wechatPaySuccess(c)
		return
	}

	LockOrder(tradeNo)
	defer UnlockOrder(tradeNo)

	topUp := model.GetTopUpByTradeNo(tradeNo)
	if topUp == nil {
		logger.LogWarn(ctx, fmt.Sprintf("微信支付 回调订单不存在 trade_no=%s transaction_id=%s client_ip=%s", tradeNo, transactionID, c.ClientIP()))
		wechatPaySuccess(c)
		return
	}
	if topUp.PaymentProvider != model.PaymentProviderWechatPay {
		logger.LogWarn(ctx, fmt.Sprintf("微信支付 订单支付网关不匹配 trade_no=%s order_provider=%s transaction_id=%s client_ip=%s", tradeNo, topUp.PaymentProvider, transactionID, c.ClientIP()))
		wechatPaySuccess(c)
		return
	}
	expectedFen := wechatPayAmountFen(topUp.Money)
	actualFen := int64(0)
	if transaction.Amount != nil && transaction.Amount.Total != nil {
		actualFen = *transaction.Amount.Total
	}
	if expectedFen > 0 && actualFen > 0 && actualFen != expectedFen {
		logger.LogError(ctx, fmt.Sprintf("微信支付 回调金额不匹配 trade_no=%s expected_fen=%d actual_fen=%d transaction_id=%s client_ip=%s", tradeNo, expectedFen, actualFen, transactionID, c.ClientIP()))
		wechatPayFail(c, http.StatusBadRequest, "订单金额不匹配")
		return
	}

	completed, quotaToAdd, err := model.CompleteWechatPayTopUp(tradeNo, transactionID, c.ClientIP())
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("微信支付 完成充值订单失败 trade_no=%s transaction_id=%s client_ip=%s error=%q", tradeNo, transactionID, c.ClientIP(), err.Error()))
		wechatPayFail(c, http.StatusInternalServerError, "订单处理失败")
		return
	}
	if quotaToAdd > 0 {
		logger.LogInfo(ctx, fmt.Sprintf("微信支付 充值成功 trade_no=%s user_id=%d transaction_id=%s client_ip=%s quota_to_add=%d money=%.2f", completed.TradeNo, completed.UserId, transactionID, c.ClientIP(), quotaToAdd, completed.Money))
		model.RecordTopupLog(completed.UserId, fmt.Sprintf("微信支付充值成功，充值额度: %v，支付金额: %.2f", logger.FormatQuota(quotaToAdd), completed.Money), c.ClientIP(), completed.PaymentMethod, model.PaymentMethodWechatPay)
	}
	wechatPaySuccess(c)
}
