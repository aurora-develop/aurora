# AURORA

[English README](https://github.com/aurora-develop/aurora/blob/main/README_EN.md)

Aurora 将 ChatGPT Web 后端能力转换为类 OpenAI API，支持聊天、Responses、文件问答、图片生成、图片变体、语音转文字、文字转语音、模型列表，以及通过 `refresh_token` / `session_token` 获取可用的 ChatGPT `access_token`。

## 接口文档

完整接口、鉴权、token 换取和 curl 示例请查看：[API.md](API.md)

## 功能

- OpenAI 风格的 `/v1/chat/completions`，支持流式和非流式返回，支持 `temperature`/`top_p`/`max_tokens`/`stop`/`reasoning_effort`/`response_format`/`stream_options.include_usage` 等参数。
- 工具调用 (Tool Calling) 模拟 — ChatGPT Web 不原生支持 function calling，Aurora 通过 `<tool_call>` 文本协议模拟该能力，支持 `tools`/`tool_choice` 字段，自动注入 system prompt 并解析模型输出中的 `<tool_call>` 块为标准 OpenAI 格式的 `tool_calls`。
- OpenAI 风格的 `/v1/responses`，支持字符串输入、消息数组、`instructions`、流式事件，以及 `reasoning.effort`/`text.query.format`/`temperature` 等参数。
- `/v1/files` 文件上传，上传后可在聊天或 Responses 请求中携带 `file_id` 做文件问答。
- `/v1/images/generations` 图片生成，模型列表包含 `gpt-image-2`，支持 SSE 流式返回，支持 URL 或 `b64_json`。
- `/v1/images/edits` 改图 + `/v1/images/variations` 图生图（变体）。
- `/v1/audio/speech` 语音合成（TTS），兼容常见 OpenAI voice 和输出格式。
- `/v1/audio/transcriptions` 语音转文字，支持 mp3/wav/m4a/ogg/flac/webm 格式。
- `/v1/audio/translations` 音频翻译为英文。
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

## 配置

默认情况无需额外配置。可以通过 `.env`、系统环境变量或同名部署平台环境变量配置：

```env
# 服务监听
SERVER_HOST=0.0.0.0
SERVER_PORT=8080
PORT=8080

# 鉴权
Authorization=your_authorization

# 免费账号
FREE_ACCOUNTS=true
FREE_ACCOUNTS_NUM=1024

# HTTPS
TLS_CERT=path_to_your_tls_cert
TLS_KEY=path_to_your_tls_key

# 代理
PROXY_URL=your_proxy_url
http_proxy=

# 转发代理（可选，给 backend-api / files 端点单独配置）
API_REVERSE_PROXY=
FILES_REVERSE_PROXY=

# 自定义 BASE_URL（默认 https://chatgpt.com/backend-api）
BASE_URL=

# 流式响应开关（设为 false 时 Chat Completions 强制返回完整响应）
STREAM_MODE=true

# 是否在请求中保留历史上下文（设为 true 启用）
ENABLE_HISTORY=false

# 工具调用模拟
TOOL_CALLING_ENABLED=true
REFUSAL_RETRIES=3
# DEBUG_TOOL_LOG=tool_debug.log
```

说明：

- `SERVER_HOST` / `SERVER_PORT`：服务监听地址和端口。`PORT` 为 `SERVER_PORT` 的 fallback。
- `Authorization`：服务访问 key。配置后，请求头需携带 `Authorization: Bearer your_authorization`。
- `FREE_ACCOUNTS`：是否自动生成免费 UUID 账号，默认关闭。
- `FREE_ACCOUNTS_NUM`：自动生成免费 UUID 账号数量，默认 1024。
- `TLS_CERT` / `TLS_KEY`：同时配置时启用 HTTPS。
- `PROXY_URL`：代理池地址。`http_proxy` 为备用代理地址。
- `API_REVERSE_PROXY` / `FILES_REVERSE_PROXY`：分别给 `/backend-api/*` 和 `/files` 端点单独配置转发代理，不配置时走默认代理。
- `BASE_URL`：自定义上游 ChatGPT API 地址，默认 `https://chatgpt.com/backend-api`。
- `STREAM_MODE`：设为 `false` 时强制关闭 Chat Completions 流式返回，默认 `true`。
- `ENABLE_HISTORY`：设为 `true` 时在请求中保留对话历史上下文。
- `TOOL_CALLING_ENABLED`：设为 `false` 时忽略请求中的 `tools` 字段，关闭工具调用模拟。
- `REFUSAL_RETRIES`：模型陷入 "sandbox 拒绝" 循环时的最大重试次数，默认 `3`。
- `DEBUG_TOOL_LOG`：设为文件路径时，将每次工具解析的详细 trace 写入该文件（调试用）。

本地账号文件：

- `access_tokens.txt`：每行一个 ChatGPT `access_token`，用于需要登录账号的能力。
- `free_tokens.txt`：每行一个 UUID device id，作为免费账号池。

## 注意事项

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
