# AURORA

已闭源发布，只用开源的可以绕行

[README_EN](https://github.com/aurora-develop/aurora/blob/main/README_EN.md)


# 免登录只支持未被oai拉黑的ip，建议用accesstoken来请求，或者在项目同目录下加入access_tokens.txt


（带UI）免费的GPT3.5，支持使用access 调用


# Web端 

访问http://你的服务器ip:8080/web

![web使用](https://jsd.cdn.zzko.cn/gh/xiaozhou26/tuph@main/images/%E5%B1%8F%E5%B9%95%E6%88%AA%E5%9B%BE%202024-04-07%20111706.png)


### 注：仅ip属地支持免登录使用ChatGpt可以使用(也可以自定义Baseurl来绕过限制)


### Docker部署
## Docker部署
您需要安装Docker和Docker Compose。

```bash
docker run -d \
  --name aurora \
  -p 8080:8080 \
  ghcr.io/aurora-develop/aurora:latest
```
## 更新容器

```bash
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock containrrr/watchtower -cR aurora --debug
```
## 现闭源发布

## Deploy

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

BASE_URL="https://chat.openai.com/backend-api" 代理网关
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
