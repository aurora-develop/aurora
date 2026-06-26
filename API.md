# Aurora 接口文档

本文档整理 Aurora 当前支持的接口、鉴权方式和 curl 示例。默认服务地址示例为 `http://你的服务器ip:8080`。

## 鉴权

受保护的 `/v1/*` 和 `/backend-api/conversation` 接口需要请求头：

```text
Authorization: Bearer <AccessToken 或 RefreshToken>
```

鉴权值可以是：

- 在环境变量 `Authorization` 中配置的服务访问 key。
- ChatGPT `access_token`，通常以 `eyJhbGciOiJSUzI1NiI` 开头。
- UUID 形式的免费 device id，仅适合普通聊天，不支持文件、图片、TTS 等需要登录账号的能力。

文件上传、文件问答、图片生成和 TTS 需要真实 ChatGPT `access_token`。你可以把多个 access token 一行一个放在项目根目录 `access_tokens.txt`，服务会轮询使用；也可以在请求头中直接传入临时 access token。

## Token 接口
如果有team账号，可以传入 ChatGPT-Account-ID，使用 Team 工作区：

Authorization 传入 ChatGPT-Account-ID值

Authorization: Bearer <AccessToken 或 RefreshToken>,<ChatGPT-Account-ID> 如果没有传入ChatGPT-Account-ID就不使用team

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

## 工具调用 (Tool Calling)

ChatGPT Web 不原生支持 OpenAI 的 function calling。Aurora 通过文本协议 `<tool_call>{...}</tool_call>` 模拟该能力：请求里声明 `tools` 字段时，Aurora 自动在 system prompt 中注入调用约定，解析模型输出中的 `<tool_call>` 块并转换为标准 OpenAI 格式的 `tool_calls`。

### 声明工具并发起调用

```bash
curl --location 'http://你的服务器ip:8080/v1/chat/completions' \
--header 'Content-Type: application/json' \
--header 'Authorization: Bearer access_token' \
--data '{
  "model": "auto",
  "messages": [
    {"role": "user", "content": "列出当前目录的文件"}
  ],
  "tools": [{
    "type": "function",
    "function": {
      "name": "bash",
      "description": "执行 shell 命令并返回输出",
      "parameters": {
        "type": "object",
        "properties": {
          "command": {"type": "string", "description": "要执行的命令"}
        },
        "required": ["command"]
      }
    }
  }]
}'
```

当模型决定调用工具时,Aurora 返回的响应 `finish_reason` 为 `tool_calls`,并在 `choices[0].message.tool_calls` 中列出调用：

```json
{
  "id": "chatcmpl-xxx",
  "object": "chat.completion",
  "choices": [{
    "index": 0,
    "message": {
      "role": "assistant",
      "content": null,
      "tool_calls": [{
        "index": 0,
        "id": "call_a1b2c3d4",
        "type": "function",
        "function": {
          "name": "bash",
          "arguments": "{\"command\":\"ls -la\"}"
        }
      }]
    },
    "finish_reason": "tool_calls"
  }],
  "usage": {"prompt_tokens": 123, "completion_tokens": 18, "total_tokens": 141}
}
```

### 把工具执行结果回传给模型

由客户端(本服务**不**执行工具)执行 `bash` 命令,得到结果,再发起新一轮请求,把结果以 `role: tool` 消息回传:

```bash
curl --location 'http://你的服务器ip:8080/v1/chat/completions' \
--header 'Content-Type: application/json' \
--header 'Authorization: Bearer access_token' \
--data '{
  "model": "auto",
  "messages": [
    {"role": "user", "content": "列出当前目录的文件"},
    {"role": "assistant", "content": null, "tool_calls": [{
      "id": "call_a1b2c3d4",
      "type": "function",
      "function": {"name": "bash", "arguments": "{\"command\":\"ls -la\"}"}
    }]},
    {"role": "tool", "tool_call_id": "call_a1b2c3d4", "name": "bash", "content": "README.md\nmain.go\n"}
  ],
  "tools": [{
    "type": "function",
    "function": {
      "name": "bash",
      "description": "执行 shell 命令并返回输出",
      "parameters": {"type": "object", "properties": {"command": {"type": "string"}}, "required": ["command"]}
    }
  }]
}'
```

模型在收到工具结果后会给出最终文字答案,`finish_reason` 为 `stop`。

### tool_choice

通过 `tool_choice` 字段控制工具调用行为,接受以下值:

- `"auto"`(默认):模型自行决定是否调用
- `"none"`:禁止调用工具
- `"any"`:强制至少调用一个工具
- `{"type":"function","function":{"name":"bash"}}`:强制调用指定工具

```json
{
  "tool_choice": {"type": "function", "function": {"name": "bash"}},
  "tools": [...]
}
```

### 环境变量

| 变量 | 默认值 | 说明 |
|---|---|---|
| `TOOL_CALLING_ENABLED` | `true` | 设为 `false` 时忽略请求中的 `tools` 字段,关闭模拟 |
| `REFUSAL_RETRIES` | `3` | 模型陷入"sandbox 隔离"拒绝循环时的最大重试次数 |
| `DEBUG_TOOL_LOG` | _(空)_ | 设为文件路径,记录每次工具解析的输入文本与解析结果(调试用) |

### 限制

- **强制非流式**:工具调用模式会强制 `stream=false`(需要完整响应才能识别 sandbox 拒绝并重试)。客户端即使传 `stream: true` 也只会拿到单次 ChatCompletion。
- **不在服务端执行工具**:Aurora 只做协议转换,工具的实际执行完全由客户端负责。
- **仅解析首个拒绝**:当前实现不会处理"工具执行失败 → 重试"循环;若需重试,由客户端再次发起完整对话。

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

### 流式返回 (SSE)

`/v1/images/generations` 和 `/v1/images/edits` 均支持 SSE 流式返回。开启方式（任一即可）：

- JSON body 传 `"stream": true`
- 查询参数 `?stream=true`
- multipart/form-data 传 `stream=true`

返回的事件序列：

```
event: image.generation.chunk
data: {"object":"image.generation.chunk","index":0,"total":1,"progress_text":"Generating image 1/1 ..."}

event: image.generation.result
data: {"object":"image.generation.result","index":0,"b64_json":"..."}

event: image.generation.completed
data: {"object":"image.generation.completed","data":[{...}]}

data: [DONE]
```

错误时返回 `image.generation.error` 事件并立即以 `data: [DONE]` 结束流。

## 改图 / 图生图

`/v1/images/edits` 同时承担两个能力：

- **改图（image edit）**：传 `prompt` + 源图，按 prompt 指示修改源图。
- **图生图（variation）**：不传 `prompt` 时，服务自动注入默认指令 `Generate a variation of the provided image(s). Return only the generated image, not a text description.`，生成与源图相似的新图。

源图可以传多张，模型会综合多张参考图理解后再生成。源图的提供方式：

- `multipart/form-data`：`image` / `image[]` / `images` / `images[]` / `image_url`（文本字段，URL 或 `data:` URL）
- `application/json`：`image_url` 字符串 / `{ "url": "..." }` 对象；`images` 数组；`image` 字段；以及 **Responses API 风格** 的 `input` / `content` / `messages`，里面含 `type: "input_image"` + `image_url`，`type: "input_text"` + `text` 会被合并成 prompt

### 改图示例

```bash
curl --location 'http://你的服务器ip:8080/v1/images/edits' \
--header 'Authorization: Bearer access_token' \
--form 'prompt="把猫改成柴犬"' \
--form 'model="gpt-image-2"' \
--form 'n=1' \
--form 'response_format="url"' \
--form 'image=@"/path/to/cat.png"'
```

### 图生图（变体）示例

```bash
curl --location 'http://你的服务器ip:8080/v1/images/edits' \
--header 'Authorization: Bearer access_token' \
--form 'n=2' \
--form 'response_format="b64_json"' \
--form 'image=@"/path/to/source.png"'
```

### Responses API 风格：多张参考图

```bash
curl --location 'http://你的服务器ip:8080/v1/images/edits' \
--header 'Content-Type: application/json' \
--header 'Authorization: Bearer access_token' \
--data '{
  "model": "gpt-image-2",
  "n": 1,
  "response_format": "url",
  "input": [
    {
      "role": "user",
      "content": [
        {"type": "input_text", "text": "把第一张图的人物放进第二张图的场景里"},
        {"type": "input_image", "image_url": "https://example.com/character.png"},
        {"type": "input_image", "image_url": "https://example.com/scene.png"}
      ]
    }
  ]
}'
```

返回结构同 `/v1/images/generations`：`{ "created": 0, "data": [{ "b64_json": "...", "revised_prompt": "..." }] }`，如使用 `response_format: "url"` 则 `b64_json` 替换为 `url`。

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

## 语音转文字 (Audio Transcriptions)

将音频文件转写为文字。与 OpenAI 官方 `/v1/audio/transcriptions` 兼容。

### 请求

```bash
curl --location 'http://你的服务器ip:8080/v1/audio/transcriptions' \
--header 'Authorization: Bearer access_token' \
--form 'file=@"/path/to/audio.mp3"' \
--form 'model="whisper-1"' \
--form 'language="zh"' \
--form 'response_format="json"'
```

| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `file` | file | 必填 | 音频文件(mp3 / wav / m4a / ogg / flac 等,上限 50MB) |
| `model` | string | `whisper-1` | 模型名 |
| `language` | string | 可选 | 语言 ISO 代码,如 `zh`、`en` |
| `prompt` | string | 可选 | 提示文本(最大 1000 字符) |
| `response_format` | string | `json` | 输出格式:`json` / `text` / `verbose_json` |
| `temperature` | float | 可选 | 采样温度 |

### 响应

`response_format=json`(默认):
```json
{
  "text": "转写后的文字内容"
}
```

`response_format=text`:
```
plain text content
```

`response_format=verbose_json`:
```json
{
  "task": "transcribe",
  "language": "zh",
  "duration": 0,
  "text": "转写后的文字内容",
  "segments": [],
  "words": []
}
```

> **注意:**`srt` 和 `vtt` 格式暂不支持(ChatGPT 后端不返回时间戳信息)。

## 音频翻译 (Audio Translations)

将音频文件翻译为英文。与 OpenAI 官方 `/v1/audio/translations` 兼容。

```bash
curl --location 'http://你的服务器ip:8080/v1/audio/translations' \
--header 'Authorization: Bearer access_token' \
--form 'file=@"/path/to/audio.mp3"' \
--form 'model="whisper-1"' \
--form 'response_format="json"'
```

参数与 Transcriptions 相同，但不接受 `language` 参数。

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
- **免费 UUID 账号（`FREE_ACCOUNTS=true`）不支持流式输出**。ChatGPT 无登录模式的流式对话走 WebSocket（`/celsius/ws/user`），该端点需要 `Authorization` header，免费账号没有 access token 会返回 401。客户端传 `stream: true` 时会自动降级为非流式返回。
- `STREAM_MODE=false` 时会强制关闭 Chat Completions 流式返回。
- 本项目是 ChatGPT Web 能力转换服务，接口形状尽量兼容 OpenAI API，但并非 OpenAI 官方服务。
