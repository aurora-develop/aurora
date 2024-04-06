FROM golang:1.21 AS builder

ENV CGO_ENABLED=0

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -ldflags "-s -w" -o /app/aurora .

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/aurora /app/aurora

EXPOSE 8080

CMD [ "./aurora" ]
