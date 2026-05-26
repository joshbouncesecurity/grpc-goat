FROM golang:1.23-alpine AS builder

ENV GOTOOLCHAIN=go1.23.4

RUN apk add --no-cache protobuf-dev
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.31.0
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0

WORKDIR /app
COPY server/ .

RUN protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    proto/service_discovery.proto \
    proto/user_directory.proto \
    proto/file_processor.proto \
    proto/image_preview.proto

RUN go mod tidy
RUN go build -o grpc-goat main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/grpc-goat .

EXPOSE 8080

CMD ["./grpc-goat"]
