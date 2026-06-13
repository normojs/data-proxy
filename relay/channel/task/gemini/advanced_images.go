package gemini

import (
	"fmt"
	"strings"
)

const (
	maxVeoReferenceImages       = 3
	defaultVeoReferenceDuration = 8
)

func ApplyVeoAdvancedImageInputs(instance *VeoInstance, params *VeoParameters, metadata map[string]any, modelName string) error {
	if instance == nil || metadata == nil {
		return nil
	}

	lastFrame, hasLastFrame, err := veoMetadataString(metadata, "lastFrame", "last_frame")
	if err != nil {
		return err
	}
	if hasLastFrame {
		if !isVeo31Model(modelName) {
			return fmt.Errorf("veo lastFrame requires a Veo 3.1 model")
		}
		if instance.Image == nil {
			return fmt.Errorf("veo lastFrame requires a primary image")
		}
		parsed := ParseImageInput(lastFrame)
		if parsed == nil {
			return fmt.Errorf("invalid veo lastFrame image")
		}
		instance.LastFrame = parsed
	}

	referenceImages, hasReferenceImages, err := parseVeoReferenceImages(metadata)
	if err != nil {
		return err
	}
	if !hasReferenceImages {
		return nil
	}
	if !isVeo31Model(modelName) {
		return fmt.Errorf("veo referenceImages requires a Veo 3.1 model")
	}
	if params == nil {
		return fmt.Errorf("veo referenceImages requires parameters")
	}
	if params.DurationSeconds == 0 {
		params.DurationSeconds = defaultVeoReferenceDuration
	} else if params.DurationSeconds != defaultVeoReferenceDuration {
		return fmt.Errorf("veo referenceImages requires durationSeconds=%d", defaultVeoReferenceDuration)
	}
	instance.ReferenceImages = referenceImages
	return nil
}

func parseVeoReferenceImages(metadata map[string]any) ([]VeoReferenceImage, bool, error) {
	raw, ok := veoMetadataValue(metadata, "referenceImages", "reference_images")
	if !ok || raw == nil {
		return nil, false, nil
	}

	items, err := veoReferenceImageItems(raw)
	if err != nil {
		return nil, true, err
	}
	if len(items) == 0 {
		return nil, false, nil
	}
	if len(items) > maxVeoReferenceImages {
		return nil, true, fmt.Errorf("veo referenceImages supports at most %d images", maxVeoReferenceImages)
	}

	referenceImages := make([]VeoReferenceImage, 0, len(items))
	for index, item := range items {
		referenceImage, err := parseVeoReferenceImage(item)
		if err != nil {
			return nil, true, fmt.Errorf("referenceImages[%d]: %w", index, err)
		}
		referenceImages = append(referenceImages, referenceImage)
	}
	return referenceImages, true, nil
}

func veoReferenceImageItems(raw any) ([]any, error) {
	switch value := raw.(type) {
	case []any:
		return value, nil
	case []string:
		items := make([]any, 0, len(value))
		for _, item := range value {
			items = append(items, item)
		}
		return items, nil
	case string:
		if strings.TrimSpace(value) == "" {
			return nil, nil
		}
		return []any{value}, nil
	default:
		return nil, fmt.Errorf("veo referenceImages must be an array or image string")
	}
}

func parseVeoReferenceImage(raw any) (VeoReferenceImage, error) {
	imageValue := ""
	referenceType := "asset"

	switch value := raw.(type) {
	case string:
		imageValue = value
	case map[string]any:
		var ok bool
		imageValue, ok = veoMapString(value, "image", "image_url", "url", "data")
		if !ok {
			return VeoReferenceImage{}, fmt.Errorf("missing image")
		}
		if parsedType, ok := veoMapString(value, "referenceType", "reference_type"); ok {
			referenceType = parsedType
		}
	case map[string]string:
		var ok bool
		imageValue, ok = veoStringMapString(value, "image", "image_url", "url", "data")
		if !ok {
			return VeoReferenceImage{}, fmt.Errorf("missing image")
		}
		if parsedType, ok := veoStringMapString(value, "referenceType", "reference_type"); ok {
			referenceType = parsedType
		}
	default:
		return VeoReferenceImage{}, fmt.Errorf("must be an image string or object")
	}

	parsed := ParseImageInput(imageValue)
	if parsed == nil {
		return VeoReferenceImage{}, fmt.Errorf("invalid image")
	}
	referenceType = strings.TrimSpace(referenceType)
	if referenceType == "" {
		referenceType = "asset"
	}
	return VeoReferenceImage{
		Image:         parsed,
		ReferenceType: referenceType,
	}, nil
}

func veoMetadataString(metadata map[string]any, keys ...string) (string, bool, error) {
	raw, ok := veoMetadataValue(metadata, keys...)
	if !ok || raw == nil {
		return "", false, nil
	}
	value, ok := raw.(string)
	if !ok {
		return "", true, fmt.Errorf("veo %s must be an image string", keys[0])
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false, nil
	}
	return value, true, nil
}

func veoMetadataValue(metadata map[string]any, keys ...string) (any, bool) {
	for _, key := range keys {
		if value, ok := metadata[key]; ok {
			return value, true
		}
	}
	return nil, false
}

func veoMapString(values map[string]any, keys ...string) (string, bool) {
	for _, key := range keys {
		value, ok := values[key]
		if !ok || value == nil {
			continue
		}
		text, ok := value.(string)
		if !ok {
			return "", false
		}
		text = strings.TrimSpace(text)
		if text != "" {
			return text, true
		}
	}
	return "", false
}

func veoStringMapString(values map[string]string, keys ...string) (string, bool) {
	for _, key := range keys {
		text := strings.TrimSpace(values[key])
		if text != "" {
			return text, true
		}
	}
	return "", false
}

func isVeo31Model(modelName string) bool {
	return strings.Contains(strings.ToLower(modelName), "veo-3.1")
}
