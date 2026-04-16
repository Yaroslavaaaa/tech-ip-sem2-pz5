FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
COPY services/auth/go.mod services/auth/go.sum ./services/auth/
COPY services/tasks/go.mod services/tasks/go.sum ./services/tasks/
COPY shared/ ./shared/
COPY pkg/ ./pkg/

RUN go mod download

COPY . .

WORKDIR /app/services/tasks
RUN go build -o tasks-service ./cmd/tasks

FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/services/tasks/tasks-service .

EXPOSE 8082

CMD ["./tasks-service"]