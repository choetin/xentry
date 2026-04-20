FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /xentry ./cmd/xentry

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /xentry /usr/local/bin/xentry
EXPOSE 8080
VOLUME /data
ENV XENTRY_DB=/data/xentry.db
ENV XENTRY_DATA_DIR=/data
CMD ["xentry"]
