package controller

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func telegramTestHash(params map[string]string, token string) string {
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, key+"="+params[key])
	}
	secret := sha256.Sum256([]byte(token))
	mac := hmac.New(sha256.New, secret[:])
	mac.Write([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestCheckTelegramAuthorizationRejectsEmptyValues(t *testing.T) {
	t.Parallel()

	require.False(t, checkTelegramAuthorization(map[string][]string{
		"hash": {},
	}, "telegram-token"))
}

func TestTelegramLoginRejectsAuthorizedRequestWithoutID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldEnabled := common.TelegramOAuthEnabled
	oldToken := common.TelegramBotToken
	common.TelegramOAuthEnabled = true
	common.TelegramBotToken = "telegram-token"
	t.Cleanup(func() {
		common.TelegramOAuthEnabled = oldEnabled
		common.TelegramBotToken = oldToken
	})

	params := map[string]string{"auth_date": "1"}
	hash := telegramTestHash(params, common.TelegramBotToken)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/oauth/telegram/login?auth_date=1&hash="+hash, nil)

	TelegramLogin(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":false`)
}
