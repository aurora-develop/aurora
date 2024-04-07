# AURORA

[README_EN](https://github.com/aurora-develop/aurora/blob/main/README_EN.md)

（带UI）免费的GPT3.5，支持使用3.5的access 调用

# 交流群
https://t.me/aurora_develop

# Web端 

![web使用](https://jsd.cdn.zzko.cn/gh/xiaozhou26/tuph@main/images/%E5%B1%8F%E5%B9%95%E6%88%AA%E5%9B%BE%202024-04-07%20111706.png)


### 注：仅ip属地支持免登录使用ChatGpt可以使用(也可以自定义Baseurl来绕过限制)

## Deploy

### Vercel部署

由于vercel不支持流式，如果在vercel部署请在STREAM_MODE中填False，不支持任何默认流式的客户端，支持沉浸式翻译。

[![Deploy with Vercel](https://vercel.com/button)](https://vercel.com/new/clone?repository-url=https%3A%2F%2Fgithub.com%2Faurora-develop%2Faurora&env=STREAM_MODE&project-name=aurora&repository-name=aurora)

### Render部署
[![Deploy](https://render.com/images/deploy-to-render-button.svg)](https://render.com/deploy)

### Koyeb部署
地区选美国

[![Deploy to Koyeb](https://www.koyeb.com/static/images/deploy/button.svg)](https://app.koyeb.com/deploy?type=docker&name=aurora&ports=8080;http;/&image=ghcr.io/aurora-develop/aurora)

### Railway部署
[![Deploy on Railway](https://railway.app/button.svg)](https://railway.app/template/jcl2Es?referralCode=XXqY_5)

### Zeabur部署
进入修改镜像名称aurora+任何字母数字

[![Deploy on Zeabur](https://zeabur.com/button.svg)](https://zeabur.com/templates/JF3EFW)

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
--data '{
     "model": "gpt-3.5-turbo",
     "messages": [{"role": "user", "content": "Say this is a test!"}],
     "stream": true
   }'
```

## 高级设置

默认情况不需要设置，除非你有需求

### 环境变量
```

BASE_URL="https://auroraxf.glitch.me/api" 代理网关
Authorization=your_authorization  用户认证 key。
TLS_CERT=path_to_your_tls_cert 存储TLS（传输层安全协议）证书的路径。
TLS_KEY=path_to_your_tls_key 存储TLS（传输层安全协议）证书的路径。
PROXY_URL=your_proxy_url 添加代理池来。
```

## 鸣谢

感谢各位大佬的pr支持，感谢。


## 参考项目


https://github.com/xqdoo00o/ChatGPT-to-API

## License

MIT License
