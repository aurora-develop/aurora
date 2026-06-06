# AURORA

[README_EN](https://github.com/aurora-develop/aurora/blob/main/README_EN.md)

## Deploy


### 编译部署

```bash
git clone https://github.com/aurora-develop/aurora
cd aurora
go build -o aurora
chmod +x ./aurora
./aurora
```

### Docker部署
## Docker部署
您需要安装Docker和Docker Compose。

```bash
docker run -d \
  --name aurora \
  -p 8080:8080 \
  ghcr.io/aurora-develop/aurora:latest
```

## Docker Compose部署
创建一个新的目录，例如aurora-app，并进入该目录：
```bash
mkdir aurora
cd aurora
```
在此目录中下载库中的docker-compose.yml文件：

```bash
docker-compose up -d
```

## Usage

```bash
curl --location 'http://你的服务器ip:8080/v1/chat/completions' \
--header 'Content-Type: application/json' \
--header 'Authorization: Bearer access_token' \
--data '{
     "model": "auto",
     "messages": [{"role": "user", "content": "Say this is a test!"}],
     "stream": true
   }'
```
### 支持codex的api

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

如需返回 base64，将 `response_format` 改为 `b64_json`。

## 高级设置

默认情况不需要设置，除非你有需求

### 环境变量
```

BASE_URL="https://chat.openai.com/backend-api" 代理网关
Authorization=your_authorization  用户认证 key。
TLS_CERT=path_to_your_tls_cert 存储TLS（传输层安全协议）证书的路径。
TLS_KEY=path_to_your_tls_key 存储TLS（传输层安全协议）证书的路径。
PROXY_URL=your_proxy_url 添加代理池来。
```

## 鸣谢

感谢各位大佬的pr支持，感谢。


## 参考项目

- [ChatGPT-to-API](https://github.com/xqdoo00o/ChatGPT-to-API)
- [chat2api](https://github.com/aurorax-neo/chat2api)

## License

MIT License

## 友链

- [linux.do](https://linux.do/)
- [xiaozhou26](https://github.com/xiaozhou26)
- [aurorax-neo](https://github.com/aurorax-neo)
