package service

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting"
)

func CheckSensitiveMessages(messages []dto.Message) ([]string, error) {
	if len(messages) == 0 {
		return nil, nil
	}

	for _, message := range messages {
		arrayContent := message.ParseContent()
		for _, m := range arrayContent {
			for _, text := range sensitiveTextsFromMediaContent(m) {
				if ok, words := SensitiveWordContains(text); ok {
					return words, errors.New("sensitive words detected")
				}
			}
		}
	}
	return nil, nil
}

func sensitiveTextsFromMediaContent(content dto.MediaContent) []string {
	switch content.Type {
	case dto.ContentTypeText:
		return nonEmptySensitiveTexts(content.Text)
	case dto.ContentTypeImageURL:
		texts := imageURLSensitiveTexts(content.ImageUrl)
		if image := content.GetImageMedia(); image != nil {
			texts = append(texts, imageURLSensitiveTexts(image)...)
		}
		return texts
	default:
		if content.Text == "" {
			return nil
		}
		return nonEmptySensitiveTexts(content.Text)
	}
}

func imageURLSensitiveTexts(imageURL any) []string {
	switch v := imageURL.(type) {
	case nil:
		return nil
	case string:
		return nonEmptySensitiveTexts(v)
	case *dto.MessageImageUrl:
		if v == nil {
			return nil
		}
		return nonEmptySensitiveTexts(v.Url, v.Detail, v.MimeType)
	case dto.MessageImageUrl:
		return nonEmptySensitiveTexts(v.Url, v.Detail, v.MimeType)
	case map[string]any:
		return collectSensitiveStringValues(v)
	case map[string]string:
		texts := make([]string, 0, len(v))
		for _, value := range v {
			texts = append(texts, nonEmptySensitiveTexts(value)...)
		}
		return texts
	case []any:
		return collectSensitiveStringValues(v)
	case []string:
		return nonEmptySensitiveTexts(v...)
	default:
		return nil
	}
}

func collectSensitiveStringValues(value any) []string {
	switch v := value.(type) {
	case string:
		return nonEmptySensitiveTexts(v)
	case []string:
		return nonEmptySensitiveTexts(v...)
	case []any:
		var texts []string
		for _, item := range v {
			texts = append(texts, collectSensitiveStringValues(item)...)
		}
		return texts
	case map[string]string:
		texts := make([]string, 0, len(v))
		for _, item := range v {
			texts = append(texts, nonEmptySensitiveTexts(item)...)
		}
		return texts
	case map[string]any:
		texts := make([]string, 0, len(v))
		for _, item := range v {
			texts = append(texts, collectSensitiveStringValues(item)...)
		}
		return texts
	default:
		return nil
	}
}

func nonEmptySensitiveTexts(values ...string) []string {
	texts := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			texts = append(texts, value)
		}
	}
	return texts
}

func CheckSensitiveText(text string) (bool, []string) {
	return SensitiveWordContains(text)
}

// SensitiveWordContains 是否包含敏感词，返回是否包含敏感词和敏感词列表
func SensitiveWordContains(text string) (bool, []string) {
	if len(setting.SensitiveWords) == 0 {
		return false, nil
	}
	if len(text) == 0 {
		return false, nil
	}
	checkText := strings.ToLower(text)
	return AcSearch(checkText, setting.SensitiveWords, true)
}

// SensitiveWordReplace 敏感词替换，返回是否包含敏感词和替换后的文本
func SensitiveWordReplace(text string, returnImmediately bool) (bool, []string, string) {
	if len(setting.SensitiveWords) == 0 {
		return false, nil, text
	}
	checkText := strings.ToLower(text)
	m := getOrBuildAC(setting.SensitiveWords)
	hits := m.MultiPatternSearch([]rune(checkText), returnImmediately)
	if len(hits) > 0 {
		words := make([]string, 0, len(hits))
		var builder strings.Builder
		builder.Grow(len(text))
		lastPos := 0

		for _, hit := range hits {
			pos := hit.Pos
			word := string(hit.Word)
			builder.WriteString(text[lastPos:pos])
			builder.WriteString("**###**")
			lastPos = pos + len(word)
			words = append(words, word)
		}
		builder.WriteString(text[lastPos:])
		return true, words, builder.String()
	}
	return false, nil, text
}
