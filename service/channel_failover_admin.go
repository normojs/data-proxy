package service

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
)

func AppendChannelFailoverAdminInfo(ctx *gin.Context, adminInfo map[string]interface{}) {
	if ctx == nil || adminInfo == nil {
		return
	}
	events, ok := common.GetContextKeyType[[]map[string]interface{}](ctx, constant.ContextKeyChannelFailoverTrace)
	if !ok || len(events) == 0 {
		return
	}
	adminInfo["channel_failover"] = events
}
