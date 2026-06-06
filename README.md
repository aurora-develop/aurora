# AURORA

[English README](https://github.com/aurora-develop/aurora/blob/main/README_EN.md)

Aurora 将 ChatGPT Web 后端能力转换为类 OpenAI API，支持聊天、Responses、文件问答、图片生成、TTS、模型列表，以及通过 `refresh_token` / `session_token` 获取可用的 ChatGPT `access_token`。

## 功能

- OpenAI 风格的 `/v1/chat/completions`，支持流式和非流式返回。
- OpenAI 风格的 `/v1/responses`，支持字符串输入、消息数组、`instructions` 和流式事件。
- `/v1/files` 文件上传，上传后可在聊天或 Responses 请求中携带 `file_id` 做文件问答。
- `/v1/images/generations` 图片生成，模型列表包含 `gpt-image-2`，支持返回 URL 或 `b64_json`。
- `/v1/audio/speech` 语音合成，兼容常见 OpenAI TTS voice 和输出格式。
- `/v1/models` 模型列表接口。
- `/auth/refresh`：传入 OpenAI `refresh_token` 获取 `access_token`。
- `/auth/session`：传入 ChatGPT `session_token` 获取新的 `session_token` 和 `access_token`。
- `/backend-api/conversation` 原始 ChatGPT conversation 请求透传。
- 支持 `access_tokens.txt` 账号池、`free_tokens.txt` 免费 UUID 池、自动生成免费账号、代理池、TLS。

## 部署

### 编译部署

```bash
git clone https://github.com/aurora-develop/aurora
cd aurora
go build -o aurora
chmod +x ./aurora
./aurora
```

### Docker 部署

```bash
docker run -d \
  --name aurora \
  -p 8080:8080 \
  ghcr.io/aurora-develop/aurora:latest
```

### Docker Compose 部署

```bash
mkdir aurora
cd aurora
# 将仓库中的 docker-compose.yml 放到当前目录后执行：
docker-compose up -d
```

## 鉴权

受保护的 `/v1/*` 和 `/backend-api/conversation` 接口需要请求头：

```text
Authorization: Bearer <你的鉴权值>
```

鉴权值可以是：

- 在环境变量 `Authorization` 中配置的服务访问 key。
- ChatGPT `access_token`，通常以 `eyJhbGciOiJSUzI1NiI` 开头。
- UUID 形式的免费 device id，仅适合普通聊天，不支持文件、图片、TTS 等需要登录账号的能力。

文件上传、文件问答、图片生成和 TTS 需要真实 ChatGPT `access_token`。你可以把多个 access token 一行一个放在项目根目录 `access_tokens.txt`，服务会轮询使用；也可以在请求头中直接传入临时 access token。

### refresh_token 换 access_token

```bash
curl --location 'http://你的服务器ip:8080/auth/refresh' \
--header 'Content-Type: application/json' \
--data '{
  "refresh_token": "你的 refresh_token"
}'
```

### session_token 换 access_token

```bash
curl --location 'http://你的服务器ip:8080/auth/session' \
--header 'Content-Type: application/json' \
--data '{
  "session_token": "你的 __Secure-next-auth.session-token"
}'
```

返回中包含 `access_token`，`/auth/session` 还会返回可用的 `session_token`。

## 接口示例

### Chat Completions

```bash
curl --location 'http://你的服务器ip:8080/v1/chat/completions' \
--header 'Content-Type: application/json' \
--header 'Authorization: Bearer access_token' \
--data '{
  "model": "auto",
  "messages": [
    {"role": "user", "content": "Say this is a test!"}
  ],
  "stream": true
}'
```

### Responses API

```bash
curl --location 'http://你的服务器ip:8080/v1/responses' \
--header 'Content-Type: application/json' \
--header 'Authorization: Bearer access_token' \
--data '{
  "model": "auto",
  "instructions": "用简洁中文回答。",
  "input": "总结一下 Aurora 支持哪些接口。",
  "stream": false
}'
```

### 模型列表

```bash
curl --location 'http://你的服务器ip:8080/v1/models' \
--header 'Authorization: Bearer access_token'
```

### 文件上传和文件问答

上传文件：

```bash
curl --location 'http://你的服务器ip:8080/v1/files' \
--header 'Authorization: Bearer access_token' \
--form 'purpose="assistants"' \
--form 'file=@"/path/to/test.pdf"'
```

使用返回的 `id` 或 `file_id` 继续问答：

```bash
curl --location 'http://你的服务器ip:8080/v1/chat/completions' \
--header 'Content-Type: application/json' \
--header 'Authorization: Bearer access_token' \
--data '{
  "model": "auto",
  "messages": [{
    "role": "user",
    "content": [
      {"type": "input_file", "file_id": "file-xxx"},
      {"type": "text", "text": "总结这个文件"}
    ]
  }],
  "stream": false
}'
```

### 图片生成

```bash
curl --location 'http://你的服务器ip:8080/v1/images/generations' \
--header 'Content-Type: application/json' \
--header 'Authorization: Bearer access_token' \
--data '{
  "model": "gpt-image-2",
  "prompt": "A cute orange cat wearing sunglasses, digital art",
  "n": 1,
  "size": "1024x1024",
  "response_format": "url"
}'
```

如需返回 base64，将 `response_format` 改为 `b64_json`。如果不传 `response_format`，默认返回 `b64_json`。

### TTS 语音合成

```bash
curl --location 'http://你的服务器ip:8080/v1/audio/speech' \
--header 'Content-Type: application/json' \
--header 'Authorization: Bearer access_token' \
--data '{
  "model": "tts-1",
  "input": "Hello, this is a test!",
  "voice": "alloy",
  "response_format": "mp3"
}' \
--output speech.mp3
```

支持的 voice 映射包括 `alloy`、`ash`、`coral`、`echo`、`fable`、`onyx`、`nova`、`sage`、`shimmer`。支持的 `response_format` 包括 `mp3`、`opus`、`aac`、`flac`、`wav`、`pcm`，其中部分格式会由上游以 AAC 形式返回。

### 原始 ChatGPT Conversation 透传

```bash
curl --location 'http://你的服务器ip:8080/backend-api/conversation' \
--header 'Content-Type: application/json' \
--header 'Authorization: Bearer access_token' \
--data '{
  "action": "next",
  "model": "auto",
  "messages": [{
    "id": "00000000-0000-0000-0000-000000000001",
    "author": {"role": "user"},
    "content": {"content_type": "text", "parts": ["hello"]}
  }],
  "parent_message_id": "00000000-0000-0000-0000-000000000000",
  "timezone_offset_min": -480,
  "history_and_training_disabled": true
}'
```

## 配置

默认情况无需额外配置。可以通过 `.env`、系统环境变量或同名部署平台环境变量配置：

```env
SERVER_HOST=0.0.0.0
SERVER_PORT=8080
FREE_ACCOUNTS=true
FREE_ACCOUNTS_NUM=1024
Authorization=your_authorization
TLS_CERT=path_to_your_tls_cert
TLS_KEY=path_to_your_tls_key
PROXY_URL=your_proxy_url
```

说明：

- `SERVER_HOST` / `SERVER_PORT`：服务监听地址和端口。
- `Authorization`：服务访问 key。配置后，请求头需携带 `Authorization: Bearer your_authorization`。
- `FREE_ACCOUNTS`：是否自动生成免费 UUID 账号，默认开启。
- `FREE_ACCOUNTS_NUM`：自动生成免费 UUID 账号数量，默认 1024。
- `TLS_CERT` / `TLS_KEY`：同时配置时启用 HTTPS。
- `PROXY_URL`：代理池地址。

本地账号文件：

- `access_tokens.txt`：每行一个 ChatGPT `access_token`，用于需要登录账号的能力。
- `free_tokens.txt`：每行一个 UUID device id，作为免费账号池。

## 注意事项

- 插件模型和 `gpt-4-plugins` 已移除，不再支持 ChatGPT Plugins。
- 图片、TTS、文件能力依赖登录态 access token，免费 UUID 账号不可用。
- `STREAM_MODE=false` 时会强制关闭 Chat Completions 流式返回。
- 本项目是 ChatGPT Web 能力转换服务，接口形状尽量兼容 OpenAI API，但并非 OpenAI 官方服务。

## 鸣谢

感谢各位大佬的 PR 支持。

## 参考项目

- [ChatGPT-to-API](https://github.com/xqdoo00o/ChatGPT-to-API)
- [chat2api](https://github.com/aurorax-neo/chat2api)

## License

MIT License

## 友链

- [linux.do](https://linux.do/)
- [xiaozhou26](https://github.com/xiaozhou26)
- [aurorax-neo](https://github.com/aurorax-neo)
