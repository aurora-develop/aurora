package imageflow

import (
	"aurora/httpclient"
	"aurora/httpclient/bogdanfinn"
	"aurora/internal/chatgpt"
	"aurora/internal/accounts"
	"encoding/base64"
	"fmt"
	"io"
	"mime/multipart"
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
func Generate(client httpclient.AuroraHttpClient, account *accounts.Account, turnStile *chatgpt.TurnStile, proxyURL string, req NormalizedImageRequest) ([]ImageResult, string, error) {
	// 上传源图（如果有）
	var references []chatgpt.ImageEditReference
	for _, src := range req.Sources {
		uploaded, _, err := chatgpt.UploadFile(client, account, proxyURL, src.Filename, src.ContentType, src.Data)
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
				client, account, turnStile, req.Prompt, req.Model, proxyURL, references,
			)
		} else {
			imageResults, upstreamText, err = chatgpt.GeneratePictureConversationImages(
				client, account, turnStile, req.Prompt, req.Model, proxyURL,
			)
		}
		if err != nil {
			return nil, upstreamText, err
		}
		if len(imageResults) == 0 && upstreamText != "" {
			return nil, upstreamText, fmt.Errorf("no image result found: %s", upstreamText)
		}
		for _, ir := range imageResults {
			item, err := convertImageResult(client, account, ir, req.ResponseFormat)
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

func convertImageResult(client httpclient.AuroraHttpClient, account *accounts.Account, ir chatgpt.ImageGenerationResult, responseFormat string) (ImageResult, error) {
	if responseFormat == "b64_json" {
		if ir.B64JSON != "" {
			return ImageResult{B64JSON: ir.B64JSON}, nil
		}
		if ir.URL != "" {
			imageBytes, err := chatgpt.DownloadImageBytes(client, ir.URL, account)
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

// ── Multipart / JSON 图片输入归一化 ──

// NormalizeMultipartImages 把 multipart.FileHeader 列表归一化为 ImageSource。
func NormalizeMultipartImages(rawImages []interface{}) []ImageSource {
	out := make([]ImageSource, 0, len(rawImages))
	for _, raw := range rawImages {
		switch v := raw.(type) {
		case *multipart.FileHeader:
			if v == nil {
				continue
			}
			f, err := v.Open()
			if err != nil {
				continue
			}
			data, err := io.ReadAll(f)
			f.Close()
			if err != nil || len(data) == 0 {
				continue
			}
			ct := v.Header.Get("Content-Type")
			if ct == "" {
				ct = "image/png"
			}
			name := v.Filename
			if name == "" {
				name = "image.png"
			}
			out = append(out, ImageSource{Data: data, Filename: name, ContentType: ct})
		case ImageSource:
			if len(v.Data) > 0 {
				out = append(out, v)
			}
		}
	}
	return out
}

// ResolveJSONImageSources 把 JSON body 中的 images/image/image_url 字段解析为 ImageSource。
func ResolveJSONImageSources(body map[string]interface{}, client httpclient.AuroraHttpClient) ([]ImageSource, error) {
	out := make([]ImageSource, 0, 2)
	appendValue := func(v interface{}) error {
		switch t := v.(type) {
		case string:
			item, ok, err := NormalizeImageURL(client, t)
			if err != nil {
				return err
			}
			if ok {
				out = append(out, item)
			}
		case map[string]interface{}:
			if urlVal, ok := t["image_url"]; ok {
				if s, ok := urlVal.(string); ok {
					item, _, err := NormalizeImageURL(client, s)
					if err != nil {
						return err
					}
					out = append(out, item)
				}
			} else if u, ok := t["url"]; ok {
				if s, ok := u.(string); ok {
					item, _, err := NormalizeImageURL(client, s)
					if err != nil {
						return err
					}
					out = append(out, item)
				}
			} else if b64, ok := t["b64_json"].(string); ok && b64 != "" {
				out = append(out, ImageSource{Data: []byte(b64), Filename: "image.png", ContentType: "image/png"})
			} else if b64, ok := t["base64"].(string); ok && b64 != "" {
				out = append(out, ImageSource{Data: []byte(b64), Filename: "image.png", ContentType: "image/png"})
			}
		}
		return nil
	}

	for _, key := range []string{"images", "image", "image_url"} {
		val, ok := body[key]
		if !ok || val == nil {
			continue
		}
		switch arr := val.(type) {
		case []interface{}:
			for _, item := range arr {
				if err := appendValue(item); err != nil {
					return nil, err
				}
			}
		case string:
			if err := appendValue(arr); err != nil {
				return nil, err
			}
		case map[string]interface{}:
			if err := appendValue(arr); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

// CollectResponsesAPIParts 从 Responses API 风格的 input / content / messages
// 字段中提取 (拼接后的文本, 所有 input_image URL 列表)。
func CollectResponsesAPIParts(raw interface{}) (string, []string) {
	if raw == nil {
		return "", nil
	}
	var textParts []string
	var imageURLs []string

	appendFromContent := func(content interface{}) {
		switch c := content.(type) {
		case string:
			if strings.TrimSpace(c) != "" {
				textParts = append(textParts, c)
			}
		case []interface{}:
			for _, item := range c {
				part, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				partType := strings.ToLower(strings.TrimSpace(stringFromAny(part["type"])))
				switch partType {
				case "input_text", "text", "output_text":
					if s := stringFromAny(part["text"]); s != "" {
						textParts = append(textParts, s)
					}
				case "input_image", "image", "image_url":
					switch u := part["image_url"].(type) {
					case string:
						imageURLs = append(imageURLs, u)
					case map[string]interface{}:
						if s := stringFromAny(u["url"]); s != "" {
							imageURLs = append(imageURLs, s)
						}
					}
					if s := stringFromAny(part["url"]); s != "" {
						imageURLs = append(imageURLs, s)
					}
				}
			}
		}
	}

	switch v := raw.(type) {
	case string:
		textParts = append(textParts, v)
	case map[string]interface{}:
		appendFromContent(v["content"])
	case []interface{}:
		for _, item := range v {
			switch m := item.(type) {
			case string:
				textParts = append(textParts, m)
			case map[string]interface{}:
				partType := strings.ToLower(strings.TrimSpace(stringFromAny(m["type"])))
				switch partType {
				case "input_text", "text", "output_text":
					if s := stringFromAny(m["text"]); s != "" {
						textParts = append(textParts, s)
					}
					continue
				case "input_image", "image", "image_url":
					switch u := m["image_url"].(type) {
					case string:
						imageURLs = append(imageURLs, u)
					case map[string]interface{}:
						if s := stringFromAny(u["url"]); s != "" {
							imageURLs = append(imageURLs, s)
						}
					}
					if s := stringFromAny(m["url"]); s != "" {
						imageURLs = append(imageURLs, s)
					}
					continue
				}
				appendFromContent(m["content"])
			}
		}
	}
	return strings.Join(textParts, "\n"), imageURLs
}

func stringFromAny(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
