

## Install

```bash
docker run -d -p 8080:8080 ghcr.io/aurora-develop/aurora:latest
```

## Usage

```bash
curl --location 'http://127.0.0.1:8080/v1/chat/completions' \
--header 'Content-Type: application/json' \
--data '{
     "model": "gpt-3.5-turbo",
     "messages": [{"role": "user", "content": "Say this is a test!"}]
   }'
```

## 参考

https://github.com/xqdoo00o/ChatGPT-to-API

## License

MIT License
