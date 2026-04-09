FROM golang:1.26 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/runner-docker-proxy ./cmd/runner-docker-proxy

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata wget

COPY --from=builder /out/runner-docker-proxy /usr/local/bin/runner-docker-proxy

EXPOSE 2375

ENTRYPOINT ["/usr/local/bin/runner-docker-proxy"]
