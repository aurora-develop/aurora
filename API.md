# Aurora 接口文档

本文档整理 Aurora 当前支持的接口、鉴权方式和 curl 示例。默认服务地址示例为 `http://你的服务器ip:8080`。

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

## Token 接口

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

## Chat Completions

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

## Responses API

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

## 模型列表

```bash
curl --location 'http://你的服务器ip:8080/v1/models' \
--header 'Authorization: Bearer access_token'
```

当前内置模型列表：

```text
auto
gpt-5
gpt-5-1
gpt-5-2
gpt-5-3
gpt-5-3-mini
gpt-image-2
```

## 文件上传和文件问答

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

## 图片生成

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

## TTS 语音合成

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

## 原始 ChatGPT Conversation 透传

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

## 注意事项

- 插件模型和 `gpt-4-plugins` 已移除，不再支持 ChatGPT Plugins。
- 图片、TTS、文件能力依赖登录态 access token，免费 UUID 账号不可用。
- `STREAM_MODE=false` 时会强制关闭 Chat Completions 流式返回。
- 本项目是 ChatGPT Web 能力转换服务，接口形状尽量兼容 OpenAI API，但并非 OpenAI 官方服务。
