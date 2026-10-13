FROM golang:1.27.2-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/erp ./cmd/erp && CGO_ENABLED=0 go build -o /out/migrate ./cmd/migrate && CGO_ENABLED=0 go build -o /out/owner ./cmd/owner
FROM alpine:3.23
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 erp && mkdir -p /data/documents && chown -R erp:erp /data
COPY --from=build /out/erp /out/migrate /out/owner /usr/local/bin/
USER erp
EXPOSE 8080
CMD ["sh", "-c", "migrate && exec erp"]
