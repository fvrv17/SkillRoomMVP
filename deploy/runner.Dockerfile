FROM golang:1.26 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/runner-service ./cmd/runner-service

FROM node:20-alpine

RUN apk add --no-cache ca-certificates tzdata wget \
 && mkdir -p /workspace \
 && chown -R node:node /workspace /opt

WORKDIR /opt/skillroom-runtime

COPY deploy/runner-runtime/package.json ./package.json
COPY deploy/runner-runtime/package-lock.json ./package-lock.json
RUN npm ci --ignore-scripts

COPY deploy/runner-runtime/run-evaluation.mjs ./run-evaluation.mjs
COPY --from=builder /out/runner-service /usr/local/bin/runner-service

USER node

EXPOSE 8081

ENTRYPOINT ["/usr/local/bin/runner-service"]
