package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
)

func TestCheckNotificationLimitMemory(t *testing.T) {
	origRedis := common.RedisEnabled
	origLimit := constant.NotifyLimitCount
	origDur := constant.NotificationLimitDurationMinute
	common.RedisEnabled = false
	constant.NotifyLimitCount = 2
	// duration must be >0; zero would expire every entry immediately
	constant.NotificationLimitDurationMinute = 60
	t.Cleanup(func() {
		common.RedisEnabled = origRedis
		constant.NotifyLimitCount = origLimit
		constant.NotificationLimitDurationMinute = origDur
	})

	uid := int(time.Now().UnixNano()%1_000_000_000) + 1
	typ := fmt.Sprintf("test_type_%d", uid)

	ok, err := CheckNotificationLimit(uid, typ)
	if err != nil || !ok {
		t.Fatalf("1st: ok=%v err=%v", ok, err)
	}
	ok, err = CheckNotificationLimit(uid, typ)
	if err != nil || !ok {
		t.Fatalf("2nd: ok=%v err=%v", ok, err)
	}
	ok, err = CheckNotificationLimit(uid, typ)
	if err != nil {
		t.Fatalf("3rd err: %v", err)
	}
	if ok {
		t.Fatal("3rd should be denied")
	}
}
