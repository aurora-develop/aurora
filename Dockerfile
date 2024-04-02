FROM golang:1.20.3-alpine as builder

ENV CGO_ENABLED=0

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o /app/Aurora .

FROM scratch

WORKDIR /app

COPY --from=builder /app/Aurora /app/Aurora

EXPOSE 8080

CMD [ "./Aurora" ]