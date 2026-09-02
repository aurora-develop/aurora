# AURORA

Aurora converts the ChatGPT Web backend into an OpenAI-style API, supporting chat, Responses, file-based Q&A, image generation, image variations, speech-to-text, text-to-speech, model listings, and obtaining a valid ChatGPT `access_token` via `refresh_token` / `session_token`.

## API Documentation

For full endpoints, authentication, token exchange, and curl examples, see: [API.md](API.md)

## Features

- OpenAI-style `/v1/chat/completions` with streaming and non-streaming support, including parameters such as `temperature`/`top_p`/`max_tokens`/`stop`/`reasoning_effort`/`response_format`/`stream_options.include_usage`.
- Tool Calling emulation — ChatGPT Web does not natively support function calling. Aurora emulates it via a `<tool_call>` text protocol, supporting `tools`/`tool_choice` fields, automatically injecting the calling convention into the system prompt and parsing `<tool_call>` blocks in model output into standard OpenAI-format `tool_calls`.
- OpenAI-style `/v1/responses` with string input, message arrays, `instructions`, streaming events, and parameters such as `reasoning.effort`/`text.query.format`/`temperature`.
- `/v1/files` for file uploads; after uploading, you can include `file_id` in chat or Responses requests for file-based Q&A.
- `/v1/images/generations` for image generation; the model list includes `gpt-image-2`, supports SSE streaming, and can return either URLs or `b64_json`.
- `/v1/images/edits` for image editing and `/v1/images/variations` for image-to-image generation (variations).
- `/v1/audio/speech` for text-to-speech (TTS), compatible with common OpenAI voices and output formats.
- `/v1/audio/transcriptions` for speech-to-text, supporting mp3/wav/m4a/ogg/flac/webm formats.
- `/v1/audio/translations` for audio translation into English.
- `/v1/models` for model listing.
- `/auth/refresh`: pass an OpenAI `refresh_token` to obtain an `access_token`.
- `/auth/session`: pass a ChatGPT `session_token` to obtain a new `session_token` and `access_token`.
- `/backend-api/conversation` for direct proxying of raw ChatGPT conversation requests.

- Support for `access_tokens.txt` account pool, `free_tokens.txt` free UUID pool, `refresh_tokens.txt` OAuth refresh token pool, `session_tokens.txt` session token pool, automatic free account generation, proxy pool, and TLS.
- **Full account management**: Each account has an independent TLS Client, proxy IP, browser fingerprint, and WSS connection. Supports 3 account types (TypeNoAuth / TypeFree / TypePUID) and 6 lifecycle states.
- **Token deduplication**: Deduplicates accounts by parsing `chatgpt_account_id` from the JWT payload — the same account won't occupy the pool twice even if multiple tokens are provided.
- **Capability system**: 10 capabilities (Chat / Responses / ToolCalling / ImageGenerate / ImageEdit / ImageVariation / TTS / Transcribe / FileUpload / WebSocket). Only chat/responses are accessible to noauth accounts; others require login.
- **Health check**: Automatically renews expired `session_token` / `refresh_token` accounts every 10 minutes.
- **External access_token support**: When `ENABLE_EXTERNAL_TOKEN=true`, accept externally provided ChatGPT access_tokens. Temporary isolated accounts (TLS Client + UA + proxy + fingerprint) are created and released after 10 minutes of inactivity.
## Deployment

### Build from source

```bash
git clone https://github.com/aurora-develop/aurora
cd aurora
go build -o aurora
chmod +x ./aurora
./aurora
```

### Docker

```bash
docker run -d \
  --name aurora \
  -p 8080:8080 \
  -v $(pwd)/access_tokens.txt:/access_tokens.txt:ro \
  ghcr.io/aurora-develop/aurora:latest
```

> Prepare `access_tokens.txt` in the current directory (one access_token per line) and mount it into the container via `-v`. The same applies to other files: `-v $(pwd)/free_tokens.txt:/free_tokens.txt:ro`, `-v $(pwd)/proxies.txt:/proxies.txt:ro`.

### Docker Compose

```bash
mkdir aurora
cd aurora
# 1. Prepare access_tokens.txt (one token per line)
# 2. Place the docker-compose.yml from the repository into the current directory
# 3. docker-compose.yml already includes a ./access_tokens.txt mount; uncomment free_tokens.txt or proxies.txt as needed
docker-compose up -d
```

## Configuration

No additional configuration is required by default. You can configure via `.env`, system environment variables, or environment variables with the same name on your deployment platform:

```env
# Server listen
SERVER_HOST=0.0.0.0
SERVER_PORT=8080
PORT=8080

# Authentication
Authorization=your_authorization

# Free accounts
FREE_ACCOUNTS=true
FREE_ACCOUNTS_NUM=1024

# HTTPS
TLS_CERT=path_to_your_tls_cert
TLS_KEY=path_to_your_tls_key

# Proxy
PROXY_URL=your_proxy_url
http_proxy=

# Reverse proxy for specific endpoints (optional)
API_REVERSE_PROXY=
FILES_REVERSE_PROXY=

# Custom BASE_URL (default https://chatgpt.com/backend-api)
BASE_URL=

# Stream mode (set to false to force non-streaming Chat Completions)
STREAM_MODE=true

# Preserve conversation history context (set to true to enable)
ENABLE_HISTORY=false

# Tool calling emulation
TOOL_CALLING_ENABLED=true
REFUSAL_RETRIES=3
# DEBUG_TOOL_LOG=tool_debug.log

# External access token (accept ChatGPT access_token from request header)
ENABLE_EXTERNAL_TOKEN=true
```

Details:

- `SERVER_HOST` / `SERVER_PORT`: Service listening address and port. `PORT` is a fallback for `SERVER_PORT`.
- `Authorization`: Service access key. When configured, requests must include `Authorization: Bearer your_authorization` in the header.
- `FREE_ACCOUNTS`: Whether to automatically generate free UUID accounts; disabled by default.
- `FREE_ACCOUNTS_NUM`: Number of automatically generated free UUID accounts; default is 1024.
- `TLS_CERT` / `TLS_KEY`: When both are configured, HTTPS is enabled.
- `PROXY_URL`: Proxy pool address. `http_proxy` is the fallback proxy address.
- `API_REVERSE_PROXY` / `FILES_REVERSE_PROXY`: Configure separate forward proxies for `/backend-api/*` and `/files` endpoints respectively; falls back to the default proxy when not set.
- `BASE_URL`: Custom upstream ChatGPT API base URL, defaults to `https://chatgpt.com/backend-api`.
- `STREAM_MODE`: Set to `false` to force non-streaming Chat Completions; defaults to `true`.
- `ENABLE_HISTORY`: Set to `true` to preserve conversation history context in requests.
- `TOOL_CALLING_ENABLED`: Set to `false` to ignore the `tools` field in requests and disable tool calling emulation.
- `REFUSAL_RETRIES`: Maximum retry attempts when the model enters a "sandbox refusal" loop; defaults to `3`.
- `ENABLE_EXTERNAL_TOKEN`: When set to `true`, accept externally provided ChatGPT access_tokens. A temporary account (with isolated TLS Client, UA, proxy, and browser fingerprint) is created per token, automatically released after 10 minutes of inactivity. Default `true`.
- `DEBUG_TOOL_LOG`: Set to a file path to write detailed trace logs for each tool call parsing (for debugging).

Local account files:

- `access_tokens.txt`: One ChatGPT `access_token` per line, used for features that require a logged-in account.
- `free_tokens.txt`: One UUID device ID per line, serving as a free account pool.
- `refresh_tokens.txt`: One OpenAI `refresh_token` per line (supports `token:team_id` format). Automatically exchanged for an `access_token` on startup; auto-renewed on expiry.
- `session_tokens.txt`: One ChatGPT `session_token` per line (supports `token:team_id` format). Automatically exchanged for an `access_token` on startup; auto-renewed on expiry.
- `proxies.txt`: One proxy URL per line (port required). Forms a proxy pool with `PROXY_URL`.
## Acknowledgments

Thanks to all the contributors for their PR support.

## Reference Projects

- [ChatGPT-to-API](https://github.com/xqdoo00o/ChatGPT-to-API)
- [chat2api](https://github.com/aurorax-neo/chat2api)

## License

MIT License

## Friendly Links

- [linux.do](https://linux.do/)
- [xiaozhou26](https://github.com/xiaozhou26)
- [aurorax-neo](https://github.com/aurorax-neo)