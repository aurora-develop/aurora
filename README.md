# AURORA

[English README](https://github.com/aurora-develop/aurora/blob/main/README_EN.md)

Aurora 将 ChatGPT Web 后端能力转换为类 OpenAI API，支持聊天、Responses、文件问答、图片生成、TTS、模型列表，以及通过 `refresh_token` / `session_token` 获取可用的 ChatGPT `access_token`。

## 接口文档

完整接口、鉴权、token 换取和 curl 示例请查看：[API.md](API.md)

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
