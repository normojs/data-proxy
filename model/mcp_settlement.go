package model

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/bytedance/gopkg/util/gopool"
)

func afterMCPToolCallQuotaSettled(userId int, tokenId int, quota int, tokenUnlimited bool) {
	if quota <= 0 {
		return
	}
	gopool.Go(func() {
		if err := cacheDecrUserQuota(userId, int64(quota)); err != nil {
			common.SysLog("failed to decrease user quota after MCP settlement: " + err.Error())
		}
	})
	if common.RedisEnabled && !tokenUnlimited && tokenId > 0 {
		gopool.Go(func() {
			token, err := GetTokenById(tokenId)
			if err != nil {
				common.SysLog("failed to load token after MCP settlement: " + err.Error())
				return
			}
			if err := cacheSetToken(*token); err != nil {
				common.SysLog("failed to refresh token cache after MCP settlement: " + err.Error())
			}
		})
	}
}

func afterMCPToolCallQuotaRefunded(userId int, tokenId int, quota int, tokenUnlimited bool) {
	if quota <= 0 {
		return
	}
	gopool.Go(func() {
		if err := cacheIncrUserQuota(userId, int64(quota)); err != nil {
			common.SysLog("failed to increase user quota after MCP refund: " + err.Error())
		}
	})
	if common.RedisEnabled && !tokenUnlimited && tokenId > 0 {
		gopool.Go(func() {
			token, err := GetTokenById(tokenId)
			if err != nil {
				common.SysLog("failed to load token after MCP refund: " + err.Error())
				return
			}
			if err := cacheSetToken(*token); err != nil {
				common.SysLog("failed to refresh token cache after MCP refund: " + err.Error())
			}
		})
	}
}
