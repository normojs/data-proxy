package controller

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
)

const systemUploadMaxBytes = 5 << 20

var allowedSystemImageTypes = map[string]string{
	"image/gif":                ".gif",
	"image/jpeg":               ".jpg",
	"image/png":                ".png",
	"image/vnd.microsoft.icon": ".ico",
	"image/webp":               ".webp",
	"image/x-icon":             ".ico",
}

func SystemUploadDir() string {
	if info, err := os.Stat("/data"); err == nil && info.IsDir() {
		return "/data/uploads/system"
	}
	return filepath.Join("data", "uploads", "system")
}

func UploadSystemLogo(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, systemUploadMaxBytes)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		common.ApiErrorMsg(c, "请选择要上传的图片文件")
		return
	}
	defer file.Close()

	if header.Size > systemUploadMaxBytes {
		common.ApiErrorMsg(c, "图片大小不能超过 5MB")
		return
	}

	head := make([]byte, 512)
	n, err := io.ReadFull(file, head)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		common.ApiErrorMsg(c, "读取图片失败")
		return
	}
	head = head[:n]
	contentType := http.DetectContentType(head)
	ext, ok := allowedSystemImageTypes[contentType]
	if !ok {
		common.ApiErrorMsg(c, "仅支持 PNG、JPG、WebP、GIF 或 ICO 图片")
		return
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		common.ApiErrorMsg(c, "读取图片失败")
		return
	}

	uploadDir := SystemUploadDir()
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		common.ApiErrorMsg(c, "创建上传目录失败")
		return
	}

	fileName := fmt.Sprintf("logo-%d%s", time.Now().UnixNano(), ext)
	targetPath := filepath.Join(uploadDir, fileName)
	out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		common.ApiErrorMsg(c, "保存图片失败")
		return
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		_ = os.Remove(targetPath)
		common.ApiErrorMsg(c, "保存图片失败")
		return
	}

	path := "/uploads/system/" + fileName
	common.ApiSuccess(c, gin.H{
		"url":  buildPublicUploadURL(c, path),
		"path": path,
	})
}

func buildPublicUploadURL(c *gin.Context, path string) string {
	base := strings.TrimRight(system_setting.ServerAddress, "/")
	if base == "" {
		scheme := "http"
		if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
			scheme = "https"
		}
		base = fmt.Sprintf("%s://%s", scheme, c.Request.Host)
	}
	return base + path
}
