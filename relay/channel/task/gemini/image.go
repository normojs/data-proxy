package gemini

import (
	"encoding/base64"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const maxVeoImageSize = 20 * 1024 * 1024 // 20 MB

// ExtractMultipartImage reads the first `input_reference` file from a multipart
// form upload and returns a VeoImageInput. Returns nil if no file is present.
func ExtractMultipartImage(c *gin.Context, info *relaycommon.RelayInfo) *VeoImageInput {
	mf, err := c.MultipartForm()
	if err != nil {
		return nil
	}
	files, exists := mf.File["input_reference"]
	if !exists || len(files) == 0 {
		return nil
	}
	fh := files[0]
	if fh.Size > maxVeoImageSize {
		return nil
	}
	file, err := fh.Open()
	if err != nil {
		return nil
	}
	defer file.Close()

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return nil
	}

	mimeType := fh.Header.Get("Content-Type")
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = http.DetectContentType(fileBytes)
	}

	info.Action = constant.TaskActionGenerate
	return &VeoImageInput{
		BytesBase64Encoded: base64.StdEncoding.EncodeToString(fileBytes),
		MimeType:           mimeType,
	}
}

// ParseImageInput parses an image string (HTTP URL, data URI, or raw base64) into a
// VeoImageInput. Returns nil if the input is empty or invalid.
func ParseImageInput(imageStr string) *VeoImageInput {
	imageStr = strings.TrimSpace(imageStr)
	if imageStr == "" {
		return nil
	}

	if strings.HasPrefix(strings.ToLower(imageStr), "http://") || strings.HasPrefix(strings.ToLower(imageStr), "https://") {
		return parseHTTPImageURL(imageStr)
	}

	if strings.HasPrefix(imageStr, "data:") {
		return parseDataURI(imageStr)
	}

	raw, err := base64.StdEncoding.DecodeString(imageStr)
	if err != nil || len(raw) > maxVeoImageSize {
		return nil
	}
	mimeType, ok := normalizeVeoImageMIME("", raw)
	if !ok {
		return nil
	}
	return &VeoImageInput{
		BytesBase64Encoded: imageStr,
		MimeType:           mimeType,
	}
}

func parseHTTPImageURL(imageURL string) *VeoImageInput {
	response, err := service.DoDownloadRequest(imageURL, "gemini_veo_image_input")
	if err != nil {
		return nil
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil
	}
	if response.ContentLength > maxVeoImageSize {
		return nil
	}
	imageBytes, err := io.ReadAll(io.LimitReader(response.Body, maxVeoImageSize+1))
	if err != nil || len(imageBytes) == 0 || len(imageBytes) > maxVeoImageSize {
		return nil
	}
	mimeType, ok := normalizeVeoImageMIME(response.Header.Get("Content-Type"), imageBytes)
	if !ok {
		return nil
	}
	return &VeoImageInput{
		BytesBase64Encoded: base64.StdEncoding.EncodeToString(imageBytes),
		MimeType:           mimeType,
	}
}

func parseDataURI(uri string) *VeoImageInput {
	// data:image/png;base64,iVBOR...
	rest := uri[len("data:"):]
	idx := strings.Index(rest, ",")
	if idx < 0 {
		return nil
	}
	meta := rest[:idx]
	b64 := rest[idx+1:]
	if b64 == "" {
		return nil
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil || len(raw) > maxVeoImageSize {
		return nil
	}

	mimeType := ""
	parts := strings.SplitN(meta, ";", 2)
	if len(parts) >= 1 && parts[0] != "" {
		mimeType = parts[0]
	}
	mimeType, ok := normalizeVeoImageMIME(mimeType, raw)
	if !ok {
		return nil
	}

	return &VeoImageInput{
		BytesBase64Encoded: b64,
		MimeType:           mimeType,
	}
}

func normalizeVeoImageMIME(headerType string, imageBytes []byte) (string, bool) {
	mimeType := strings.TrimSpace(headerType)
	if mimeType != "" {
		if parsed, _, err := mime.ParseMediaType(mimeType); err == nil {
			mimeType = parsed
		}
	}
	if mimeType == "" || mimeType == "application/octet-stream" || !isSupportedVeoImageMIME(mimeType) {
		mimeType = http.DetectContentType(imageBytes)
	}
	if !isSupportedVeoImageMIME(mimeType) {
		return "", false
	}
	return mimeType, true
}

func isSupportedVeoImageMIME(mimeType string) bool {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/jpeg", "image/png", "image/webp", "image/gif":
		return true
	default:
		return false
	}
}
