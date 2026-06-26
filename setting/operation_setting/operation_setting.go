package operation_setting

import "strings"

var DemoSiteEnabled = false
var SelfUseModeEnabled = false

var AutomaticDisableKeywords = []string{
	"Your credit balance is too low",
	"This organization has been disabled.",
	"You exceeded your current quota",
	"Permission denied",
	"The security token included in the request is invalid",
	"Operation not allowed",
	"Your account is not authorized",
}

var ChannelHealthTransientKeywords = []string{
	"timeout",
	"deadline exceeded",
	"connection reset",
	"connection refused",
	"connection closed",
	"no such host",
	"eof",
	"unexpected end",
	"tls handshake",
	"temporary failure",
}

func AutomaticDisableKeywordsToString() string {
	return strings.Join(AutomaticDisableKeywords, "\n")
}

func AutomaticDisableKeywordsFromString(s string) {
	AutomaticDisableKeywords = []string{}
	ak := strings.Split(s, "\n")
	for _, k := range ak {
		k = strings.TrimSpace(k)
		k = strings.ToLower(k)
		if k != "" {
			AutomaticDisableKeywords = append(AutomaticDisableKeywords, k)
		}
	}
}

func ChannelHealthTransientKeywordsToString() string {
	return strings.Join(ChannelHealthTransientKeywords, "\n")
}

func ChannelHealthTransientKeywordsFromString(s string) {
	ChannelHealthTransientKeywords = []string{}
	keywords := strings.Split(s, "\n")
	for _, k := range keywords {
		k = strings.TrimSpace(k)
		k = strings.ToLower(k)
		if k != "" {
			ChannelHealthTransientKeywords = append(ChannelHealthTransientKeywords, k)
		}
	}
}
