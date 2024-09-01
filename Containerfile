FROM docker.io/golang:1.22-alpine AS builder
RUN apk update && apk add --no-cache git
WORKDIR /app/src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -ldflags="-w -s" -o /app/tgmarkovgo

FROM docker.io/alpine:latest
COPY --from=builder /app/tgmarkovgo /app/tgmarkovgo
WORKDIR /app
CMD [ "/app/tgmarkovgo" ]