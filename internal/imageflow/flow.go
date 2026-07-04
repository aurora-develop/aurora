package imageflow

import (
	"aurora/httpclient"
	"aurora/httpclient/bogdanfinn"
	"aurora/internal/chatgpt"
	"aurora/internal/tokens"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ImageSource 表示一张待处理的源图。
type ImageSource struct {
	Data        []byte
	Filename    string
	ContentType string
}

// NormalizedImageRequest 归一化后的图片请求。
type NormalizedImageRequest struct {
	Prompt         string
	Model          string
	ResponseFormat string
	N              int
	Stream         bool
	Sources        []ImageSource
	Variation      bool
}

// ImageResult 单张图片的生成结果。
type ImageResult struct {
	URL     string
	B64JSON string
}

// Generate 执行图片生成流程：上传源图 → 调用上游 → 转换结果。
func Generate(client httpclient.AuroraHttpClient, secret *tokens.Secret, turnStile *chatgpt.TurnStile, proxyURL string, req NormalizedImageRequest) ([]ImageResult, string, error) {
	// 上传源图（如果有）
	var references []chatgpt.ImageEditReference
	for _, src := range req.Sources {
		uploaded, _, err := chatgpt.UploadFile(client, secret, proxyURL, src.Filename, src.ContentType, src.Data)
		if err != nil {
			return nil, "", fmt.Errorf("upload image failed: %w", err)
		}
		references = append(references, chatgpt.ImageEditReference{
			FileID:   uploaded.FileID,
			Width:    uploaded.Width,
			Height:   uploaded.Height,
			Size:     int(uploaded.Bytes),
			MimeType: uploaded.MimeType,
			Filename: uploaded.Filename,
		})
	}

	// 调用上游生成
	var allResults []ImageResult
	var upstreamText string
	for i := 0; i < req.N; i++ {
		var imageResults []chatgpt.ImageGenerationResult
		var err error
		if len(references) > 0 {
			imageResults, upstreamText, err = chatgpt.GeneratePictureConversationImagesWithReferences(
				client, secret, turnStile, req.Prompt, req.Model, proxyURL, references,
			)
		} else {
			imageResults, upstreamText, err = chatgpt.GeneratePictureConversationImages(
				client, secret, turnStile, req.Prompt, req.Model, proxyURL,
			)
		}
		if err != nil {
			return nil, upstreamText, err
		}
		if len(imageResults) == 0 && upstreamText != "" {
			return nil, upstreamText, fmt.Errorf("no image result found: %s", upstreamText)
		}
		for _, ir := range imageResults {
			item, err := convertImageResult(client, secret, ir, req.ResponseFormat)
			if err != nil {
				return nil, upstreamText, err
			}
			allResults = append(allResults, item)
			if len(allResults) >= req.N {
				break
			}
		}
		if len(allResults) >= req.N {
			break
		}
	}
	return allResults, upstreamText, nil
}

func convertImageResult(client httpclient.AuroraHttpClient, secret *tokens.Secret, ir chatgpt.ImageGenerationResult, responseFormat string) (ImageResult, error) {
	if responseFormat == "b64_json" {
		if ir.B64JSON != "" {
			return ImageResult{B64JSON: ir.B64JSON}, nil
		}
		if ir.URL != "" {
			imageBytes, err := chatgpt.DownloadImageBytes(client, ir.URL, secret)
			if err != nil {
				return ImageResult{}, fmt.Errorf("image download failed: %w", err)
			}
			return ImageResult{B64JSON: base64.StdEncoding.EncodeToString(imageBytes)}, nil
		}
	}
	return ImageResult{URL: ir.URL, B64JSON: ir.B64JSON}, nil
}

// NormalizeImageURL 解析单个 image_url 字符串为 ImageSource。
func NormalizeImageURL(client httpclient.AuroraHttpClient, raw string) (ImageSource, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ImageSource{}, false, nil
	}
	if strings.HasPrefix(raw, "data:") {
		return decodeDataURL(raw)
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return downloadHTTPURL(client, raw)
	}
	return ImageSource{Data: []byte(raw), Filename: "image.png", ContentType: "image/png"}, true, nil
}

func decodeDataURL(url string) (ImageSource, bool, error) {
	comma := strings.Index(url, ",")
	if comma < 0 {
		return ImageSource{}, false, fmt.Errorf("invalid data URL")
	}
	header := url[:comma]
	payload := url[comma+1:]
	contentType := "image/png"
	if semi := strings.Index(header, ";"); semi > 5 {
		contentType = header[5:semi]
	}
	dec := base64.StdEncoding
	if strings.Contains(header, ";base64") {
		dec = base64.StdEncoding
	} else {
		dec = base64.URLEncoding
	}
	raw, err := dec.DecodeString(payload)
	if err != nil {
		raw, err = base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return ImageSource{}, false, err
		}
	}
	ext := "png"
	switch strings.ToLower(contentType) {
	case "image/jpeg":
		ext = "jpg"
	case "image/gif":
		ext = "gif"
	case "image/webp":
		ext = "webp"
	}
	return ImageSource{Data: raw, Filename: "image_url." + ext, ContentType: contentType}, true, nil
}

func downloadHTTPURL(client httpclient.AuroraHttpClient, url string) (ImageSource, bool, error) {
	if client == nil {
		client = bogdanfinn.NewStdClient()
	}
	headers := httpclient.AuroraHeaders{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
		"Accept":     "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8",
	}
	resp, err := client.Request(httpclient.GET, url, headers, nil, nil)
	if err != nil {
		return ImageSource{}, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ImageSource{}, false, fmt.Errorf("download image failed: status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return ImageSource{}, false, err
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	filename := "image_url"
	if idx := strings.LastIndex(url, "/"); idx >= 0 && idx < len(url)-1 {
		filename = url[idx+1:]
	}
	if dot := strings.Index(filename, "?"); dot >= 0 {
		filename = filename[:dot]
	}
	if filename == "" {
		filename = "image_url.png"
	}
	return ImageSource{Data: data, Filename: filename, ContentType: contentType}, true, nil
}
