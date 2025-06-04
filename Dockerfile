FROM golang:1.23.4-alpine AS builder

ENV CGO_ENABLED=0 GOOS=linux

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN go build -o /go-ecommerce ./main.go

FROM alpine:latest

WORKDIR /root/

COPY --from=builder /go-ecommerce .

EXPOSE 8080

CMD ["./go-ecommerce"]