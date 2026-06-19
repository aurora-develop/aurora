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

## License

MIT License
