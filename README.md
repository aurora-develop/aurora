# AURORA

## 环境变量
```bash
SERVER_HOST=0.0.0.0 服务器监听的IP地址。
SERVER_PORT=8080 服务器监听的端口。
FREE_ACCOUNTS=true 是否允许创建免费账户。
FREE_ACCOUNTS_NUM=1024 允许创建的免费账户数。
Authorization= 用户认证 Authorization。
TLS_CERT= 存储TLS（传输层安全协议）证书的路径。
TLS_KEY= 存储TLS证书的私钥的路径。
PROXY_URL= 添加代理池来使用免费3.5。

```

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
  -e SERVER_HOST=0.0.0.0 \
  -e SERVER_PORT=8080 \
  -e FREE_ACCOUNTS=true \
  -e FREE_ACCOUNTS_NUM=1024 \
  -e Authorization=<your_authorization> \
  -e TLS_CERT=<path_to_your_tls_cert> \
  -e TLS_KEY=<path_to_your_tls_key> \
  -e PROXY_URL=<your_proxy_url> \
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
curl --location 'http://127.0.0.1:8080/v1/chat/completions' \
--header 'Content-Type: application/json' \
--data '{
     "model": "gpt-3.5-turbo",
     "messages": [{"role": "user", "content": "Say this is a test!"}],
     "stream": true
   }'
```

## 鸣谢

感谢各位大佬的pr支持，感谢。

## 参考

https://github.com/xqdoo00o/ChatGPT-to-API

## License

MIT License
