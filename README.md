

## Deploy

### 编译部署

```bash
go build -o aurora
chmod +x ./myapp
./myapp
```

### Docker部署

```bash
docker run -d --name aurora -p 8080:8080 ghcr.io/aurora-develop/aurora:latest
```

## Usage

```bash
curl --location 'http://127.0.0.1:8080/v1/chat/completions' \
--header 'Content-Type: application/json' \
--data '{
     "model": "gpt-3.5-turbo",
     "messages": [{"role": "user", "content": "Say this is a test!"}],
     "stream": true
   }'
```

## 贡献

https://github.com/xqdoo00o/ChatGPT-to-API

## License

MIT License
