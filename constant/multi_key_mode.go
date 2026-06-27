package constant

type MultiKeyMode string

const (
	MultiKeyModeRandom            MultiKeyMode = "random"              // 随机
	MultiKeyModePolling           MultiKeyMode = "polling"             // 轮询
	MultiKeyModeStickyHashBounded MultiKeyMode = "sticky_hash_bounded" // 负载保护粘性分配
)

func IsValidMultiKeyMode(mode MultiKeyMode) bool {
	switch mode {
	case MultiKeyModeRandom, MultiKeyModePolling, MultiKeyModeStickyHashBounded:
		return true
	default:
		return false
	}
}

func NormalizeMultiKeyMode(mode MultiKeyMode) MultiKeyMode {
	if IsValidMultiKeyMode(mode) {
		return mode
	}
	return MultiKeyModeRandom
}
