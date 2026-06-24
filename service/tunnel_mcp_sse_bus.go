package service

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const tunnelMCPSSERedisChannelPrefix = "tunnel:mcp:sse:"

type tunnelMCPSSEBus interface {
	Publish(sessionId string, body []byte) bool
	Subscribe(sessionId string, handler func([]byte)) (func(), error)
}

var defaultTunnelMCPSSEBus tunnelMCPSSEBus = redisTunnelMCPSSEBus{}

func setTunnelMCPSSEBusForTest(bus tunnelMCPSSEBus) func() {
	previous := defaultTunnelMCPSSEBus
	if bus == nil {
		bus = noopTunnelMCPSSEBus{}
	}
	defaultTunnelMCPSSEBus = bus
	return func() {
		defaultTunnelMCPSSEBus = previous
	}
}

type noopTunnelMCPSSEBus struct{}

func (noopTunnelMCPSSEBus) Publish(_ string, _ []byte) bool {
	return false
}

func (noopTunnelMCPSSEBus) Subscribe(_ string, _ func([]byte)) (func(), error) {
	return nil, nil
}

type redisTunnelMCPSSEBus struct{}

func (redisTunnelMCPSSEBus) Publish(sessionId string, body []byte) bool {
	if !tunnelMCPSSEBusEnabled() {
		return false
	}
	sessionId = strings.TrimSpace(sessionId)
	if sessionId == "" || len(body) == 0 {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	subscribers, err := common.RDB.Publish(ctx, tunnelMCPSSEBusChannel(sessionId), body).Result()
	if err != nil {
		common.SysLog("tunnel mcp sse redis publish failed: " + err.Error())
		return false
	}
	return subscribers > 0
}

func (redisTunnelMCPSSEBus) Subscribe(sessionId string, handler func([]byte)) (func(), error) {
	if !tunnelMCPSSEBusEnabled() {
		return nil, nil
	}
	sessionId = strings.TrimSpace(sessionId)
	if sessionId == "" || handler == nil {
		return nil, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	pubsub := common.RDB.Subscribe(ctx, tunnelMCPSSEBusChannel(sessionId))
	readyCtx, readyCancel := context.WithTimeout(ctx, 3*time.Second)
	_, err := pubsub.Receive(readyCtx)
	readyCancel()
	if err != nil {
		cancel()
		_ = pubsub.Close()
		return nil, err
	}
	go func() {
		for msg := range pubsub.Channel() {
			if msg == nil {
				continue
			}
			handler([]byte(msg.Payload))
		}
	}()
	var once sync.Once
	return func() {
		once.Do(func() {
			cancel()
			_ = pubsub.Close()
		})
	}, nil
}

func tunnelMCPSSEBusEnabled() bool {
	return common.RedisEnabled && common.RDB != nil
}

func tunnelMCPSSEBusChannel(sessionId string) string {
	return tunnelMCPSSERedisChannelPrefix + sessionId
}
