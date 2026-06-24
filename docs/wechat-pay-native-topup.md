# WeChat Pay Native Topup

data-proxy supports direct WeChat Pay API v3 Native QR-code topups for wallet recharge. This is separate from the legacy Epay `wxpay` method.

## Required Configuration

Configure these options in the admin payment gateway settings, or write the same option keys into the database:

- `WechatPayEnabled`: `true`
- `WechatPayAppID`: the AppID bound to the WeChat Pay merchant account
- `WechatPayMchID`: WeChat Pay merchant ID
- `WechatPayAPIv3Key`: 32-byte API v3 key
- `WechatPayMerchantSerialNo`: merchant API certificate serial number
- `WechatPayPrivateKeyPath`: container path to `apiclient_key.pem`; file path is preferred
- `WechatPayPrivateKey`: optional inline private key fallback; leave blank for production secret-file deployments
- `WechatPayNotifyUrl`: optional override; otherwise `${CustomCallbackAddress || ServerAddress}/api/wechat-pay/notify`
- `WechatPayProductName`: optional QR order description prefix
- `WechatPayMinTopUp`: minimum topup amount, default `1`

The public callback URL must be HTTPS and reachable by WeChat Pay. For the current deployment this should normally be:

```text
https://dp.app.mbu.ltd/api/wechat-pay/notify
```

Also make sure `ServerAddress` or `CustomCallbackAddress` points to the public HTTPS domain, and that server time is synchronized.

## Merchant Private Key Management

Production deployments should keep the WeChat Pay merchant private key out of Git and out of database option values. The provided `docker-compose.wechat-pay.yml` override mounts this host directory as read-only:

```text
./secrets/wechatpay -> /run/secrets/data-proxy/wechatpay
```

On the server, place the merchant private key next to `docker-compose.prod.yml`:

```bash
mkdir -p secrets/wechatpay
install -m 600 /path/to/apiclient_key.pem secrets/wechatpay/apiclient_key.pem
chmod 700 secrets secrets/wechatpay
```

Start or restart the service through the production wrapper. It always includes
the WeChat Pay compose override:

```bash
scripts/prod-compose.sh up -d data-proxy
```

For image deployments, prefer `scripts/prod-deploy.sh`. It also includes the
WeChat Pay override and archives the currently running image before switching:

```bash
scripts/prod-deploy.sh ./data-proxy-<tag>.tar
```

Then configure these fields in the data-proxy admin payment gateway settings:

```text
WechatPayPrivateKeyPath=/run/secrets/data-proxy/wechatpay/apiclient_key.pem
WechatPayPrivateKey=
```

When both fields are set, `WechatPayPrivateKeyPath` takes precedence so certificate rotation can be handled by replacing the mounted file and restarting the service.

## Runtime Flow

1. User selects `WeChat Pay` in Wallet.
2. Backend creates a `wechat_pay` pending `TopUp` order.
3. Backend calls WeChat Pay Native `Prepay` and returns `code_url`.
4. Frontend renders the QR code.
5. WeChat Pay posts the encrypted notification to `/api/wechat-pay/notify`.
6. Backend verifies the signature, decrypts the notification, validates amount, marks the topup successful, credits quota, and writes a wallet ledger event.
