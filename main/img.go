package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/nfnt/resize"
)

// =====================
// 数据结构
// =====================
type Metadata struct {
	Image      string
	ImageThumb []byte
	Height     *uint32
	Width      *uint32
}

// =====================
// 对外入口
// =====================
func GetImageAndSave(imgUrl string, dir string) (Metadata, string, error) {
	var meta Metadata
	meta.Image = imgUrl

	baseName := generateFileName(imgUrl)
	filePath := filepath.Join(dir, baseName+".jpg") // ✅ 强制 jpg

	// 1️⃣ 尝试读取缓存
	if data, err := os.ReadFile(filePath); err == nil {
		if len(data) > 0 {
			meta.ImageThumb = data
			return meta, filePath, nil
		}
		// 空文件 → 删除
		_ = os.Remove(filePath)
	}

	// 2️⃣ 创建目录
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return meta, "", err
	}

	// 3️⃣ 下载 + 处理
	meta, err := downloadAndMakeThumb(imgUrl)
	if err != nil {
		return meta, "", err
	}

	// 4️⃣ 写缓存
	_ = os.WriteFile(filePath, meta.ImageThumb, 0644)

	return meta, filePath, nil
}

// =====================
// 下载 + 生成 WA 缩略图（核心）
// =====================
func downloadAndMakeThumb(imgUrl string) (Metadata, error) {
	meta := Metadata{Image: imgUrl}

	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	resp, err := client.Get(imgUrl)
	if err != nil {
		return meta, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return meta, fmt.Errorf("http error: %s", resp.Status)
	}

	reader := io.LimitReader(resp.Body, 10*1024*1024)
	data, err := io.ReadAll(reader)
	if err != nil {
		return meta, err
	}

	if len(data) == 0 {
		return meta, fmt.Errorf("empty image data")
	}

	// =====================
	// 解码图片
	// =====================
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		// ❌ 非图片 → 不要用
		return meta, fmt.Errorf("not an image")
	}

	// =====================
	// 获取原始尺寸
	// =====================
	bounds := img.Bounds()
	w := uint32(bounds.Dx())
	h := uint32(bounds.Dy())
	meta.Width = &w
	meta.Height = &h

	// =====================
	// 最好是宽高比3:2
	// =====================
	thumbImg := resize.Thumbnail(300, 200, img, resize.Lanczos3)

	// =====================
	// 强制 JPEG（WA只认这个）
	// =====================
	var buf bytes.Buffer
	err = jpeg.Encode(&buf, thumbImg, &jpeg.Options{
		Quality: 90, // ⚠️ 关键：控制大小
	})
	if err != nil {
		return meta, err
	}

	thumb := buf.Bytes()

	// =====================
	// 大小保护（WA要求很严格）
	// =====================
	if len(thumb) > 200*1024 {
		// 再压一次
		buf.Reset()
		_ = jpeg.Encode(&buf, thumbImg, &jpeg.Options{Quality: 60})
		thumb = buf.Bytes()
	}

	// 再兜底
	if len(thumb) > 20*1024 {
		return meta, fmt.Errorf("thumbnail too large")
	}

	meta.ImageThumb = thumb

	return meta, nil
}

// =====================
// 生成文件名
// =====================
func generateFileName(url string) string {
	hash := md5.Sum([]byte(url))
	return hex.EncodeToString(hash[:])
}
