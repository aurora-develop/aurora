# AURORA

Supports the use of GPT-3.5 through access calls.

# Communication Group
https://t.me/aurora_develop

## Deployment

### Deployment on Render
[![Deploy](https://render.com/images/deploy-to-render-button.svg)](https://render.com/deploy)

### Deployment on Koyeb
Choose United States as the region.

[![Deploy to Koyeb](https://www.koyeb.com/static/images/deploy/button.svg)](https://app.koyeb.com/deploy?type=docker&name=aurora&ports=8080;http;/&image=ghcr.io/aurora-develop/aurora)

### Deployment on Railway
[![Deploy on Railway](https://railway.app/button.svg)](https://railway.app/template/jcl2Es?referralCode=XXqY_5)

### Deployment on Zeabur
Go in and change the image name to aurora plus any letter or number.

[![Deploy on Zeabur](https://zeabur.com/button.svg)](https://zeabur.com/templates/JF3EFW)

### Compilation Deployment

```bash
git clone https://github.com/aurora-develop/aurora
cd aurora
go build -o aurora
chmod +x ./aurora
./aurora
```

### Docker Deployment
You need to install Docker and Docker Compose.

```bash
docker run -d \
  --name aurora \
  -p 8080:8080 \
  ghcr.io/aurora-develop/aurora:latest
```

## Docker Compose Deployment
Create a new directory, for example, aurora-app, and enter that directory:
```bash
mkdir aurora
cd aurora
```
In this directory, download the docker-compose.yml file from the repository:

```bash
docker-compose up -d
```

## Usage

```bash
curl --location 'http://your-server-ip:8080/v1/chat/completions' \
--header 'Content-Type: application/json' \
--data '{
     "model": "gpt-3.5-turbo",
     "messages": [{"role": "user", "content": "Say this is a test!"}],
     "stream": true
   }'
```

## Advanced Settings

Default settings do not need to be changed unless you have specific needs.

### Environment Variables
```
BASE_URL="https://auroraxf.glitch.me/api" Proxy gateway.
Authorization=your_authorization User authentication key.
TLS_CERT=path_to_your_tls_cert Path to your TLS (Transport Layer Security) certificate.
TLS_KEY=path_to_your_tls_key Path to your TLS (Transport Layer Security) key.
PROXY_URL=your_proxy_url Add a proxy pool.
```

## Acknowledgements

Thanks to all the great developers for their PR support, thank you.

## Reference Projects

https://github.com/xqdoo00o/ChatGPT-to-API

## Tool Calling (Function Calling)

ChatGPT Web does **not** natively support OpenAI-style function calling. Aurora
emulates it via a text protocol inspired by [chatgptproxy](https://github.com/acruz6421-bot/chatgptproxy):

- The system prompt is augmented with `<tool_call>{"name": "...", "arguments": {...}}</tool_call>`
  instructions describing the available tools.
- Streaming responses are parsed live to extract `<tool_call>` blocks.
- The response is converted to standard OpenAI format with `tool_calls[]` populated.

### Enabling

Send a standard OpenAI-format request that includes a `tools` field. Aurora detects
the field and switches to tool-calling mode automatically.

```bash
curl --location 'http://your-server-ip:8080/v1/chat/completions' \
--header 'Content-Type: application/json' \
--data '{
  "model": "auto",
  "messages": [{"role": "user", "content": "List the files in the current directory."}],
  "tools": [{
    "type": "function",
    "function": {
      "name": "bash",
      "description": "Run a shell command",
      "parameters": {
        "type": "object",
        "properties": {
          "command": {"type": "string", "description": "Shell command to execute"}
        },
        "required": ["command"]
      }
    }
  }]
}'
```

When the model wants to call a tool, Aurora returns:

```json
{
  "choices": [{
    "message": {
      "role": "assistant",
      "content": null,
      "tool_calls": [{
        "index": 0,
        "id": "call_xxxxxxxx",
        "type": "function",
        "function": {
          "name": "bash",
          "arguments": "{\"command\":\"ls -la\"}"
        }
      }]
    },
    "finish_reason": "tool_calls"
  }]
}
```

Run the tool on your side, then send a follow-up request with the result:

```json
{
  "messages": [
    {"role": "user", "content": "List the files"},
    {"role": "assistant", "tool_calls": [{"id": "call_xxxxxxxx", "type": "function", "function": {"name": "bash", "arguments": "{\"command\":\"ls -la\"}"}}]},
    {"role": "tool", "tool_call_id": "call_xxxxxxxx", "name": "bash", "content": "file1.py\nfile2.py"}
  ],
  "tools": [...]
}
```

The model will produce its final text answer on the next turn.

### Configuration

| Variable | Default | Effect |
|---|---|---|
| `TOOL_CALLING_ENABLED` | `true` | Set to `false` to disable the emulation. Requests with `tools` will be passed through as plain chat. |
| `REFUSAL_RETRIES` | `3` | Max retries when the model falls into the "isolated sandbox" refusal loop. Each retry appends a stronger prompt to force a `<tool_call>` emission. |
| `DEBUG_TOOL_LOG` | _(empty)_ | Path to a file where each tool-parse attempt is logged (raw text + extracted calls). Set this for debugging only. |

### Limitations

- **Streaming**: tool-calling mode forces `stream=false` internally so that sandbox-rejection retries can buffer the full response. The returned `ChatCompletion` is non-streamed.
- **No tool execution**: Aurora does **not** execute tools on your behalf. Your client must run the tool and feed the result back via `role: tool` messages.
- **Tool schema**: Aurora only injects `name`, `description`, and `parameters` into the system prompt. Other OpenAI fields (`strict`, `cache_control`, etc.) are ignored.

## License

MIT License
