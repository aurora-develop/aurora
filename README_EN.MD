# AURORA

A free GPT-3.5 API

### Note: Only IPs from supported regions can use ChatGPT without logging in

## Deploy

### Render Deployment
[![Deploy](https://render.com/images/deploy-to-render-button.svg)](https://render.com/deploy)

### Compilation Deployment

```bash
git clone https://github.com/aurora-develop/aurora
cd aurora
go build -o aurora
chmod +x ./aurora
./aurora
```

### Docker Deployment
## Docker Deployment
You need to install Docker and Docker Compose.

```bash
docker run -d \
  --name aurora \
  -p 8080:8080 \
  ghcr.io/aurora-develop/aurora:latest
```

## Docker Compose Deployment
Create a new directory, for example, aurora-app, and enter it:
```bash
mkdir aurora
cd aurora
```
In this directory, download the docker-compose.yml file from the library:

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

## Advanced Settings

Default settings do not need to be adjusted unless you have specific requirements

### Environment Variables
```
Authorization=your_authorization
TLS_CERT=path_to_your_tls_cert
TLS_KEY=path_to_your_tls_key
PROXY_URL=your_proxy_url
```

## Acknowledgments

Thank you for the PR support from various contributors, much appreciated.

## Reference Projects

https://github.com/xqdoo00o/ChatGPT-to-API

## License

MIT License
