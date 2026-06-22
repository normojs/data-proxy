# WeChat Pay Native Topup

data-proxy supports direct WeChat Pay API v3 Native QR-code topups for wallet recharge. This is separate from the legacy Epay `wxpay` method.

## Required Configuration

Configure these options in the admin payment gateway settings, or write the same option keys into the database:

- `WechatPayEnabled`: `true`
- `WechatPayAppID`: the AppID bound to the WeChat Pay merchant account
- `WechatPayMchID`: WeChat Pay merchant ID
- `WechatPayAPIv3Key`: 32-byte API v3 key
- `WechatPayMerchantSerialNo`: merchant API certificate serial number
- `WechatPayPrivateKeyPath`: path to `apiclient_key.pem`
- `WechatPayPrivateKey`: optional inline private key; used when `WechatPayPrivateKeyPath` is empty
- `WechatPayNotifyUrl`: optional override; otherwise `${CustomCallbackAddress || ServerAddress}/api/wechat-pay/notify`
- `WechatPayProductName`: optional QR order description prefix
- `WechatPayMinTopUp`: minimum topup amount, default `1`

The public callback URL must be HTTPS and reachable by WeChat Pay. For the current deployment this should normally be:

```text
https://dp.app.mbu.ltd/api/wechat-pay/notify
```

Also make sure `ServerAddress` or `CustomCallbackAddress` points to the public HTTPS domain, and that server time is synchronized.

## Runtime Flow

1. User selects `WeChat Pay` in Wallet.
2. Backend creates a `wechat_pay` pending `TopUp` order.
3. Backend calls WeChat Pay Native `Prepay` and returns `code_url`.
4. Frontend renders the QR code.
5. WeChat Pay posts the encrypted notification to `/api/wechat-pay/notify`.
6. Backend verifies the signature, decrypts the notification, validates amount, marks the topup successful, credits quota, and writes a wallet ledger event.
