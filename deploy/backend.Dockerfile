FROM golang:1.26 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/backend ./cmd/backend

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata wget && adduser -D -u 10001 appuser

WORKDIR /app

COPY --from=builder /out/backend /usr/local/bin/backend

USER appuser

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/backend"]
