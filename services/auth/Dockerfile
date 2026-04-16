FROM golang:1.24-alpine AS builder

WORKDIR /app

# Копируем все go.mod файлы из корня проекта
COPY go.mod go.sum ./
COPY services/auth/go.mod services/auth/go.sum ./services/auth/
COPY services/tasks/go.mod services/tasks/go.sum ./services/tasks/
COPY shared/ ./shared/
COPY pkg/ ./pkg/

# Устанавливаем зависимости для корневого модуля
RUN go mod download

# Копируем весь исходный код
COPY . .

# Переходим в директорию auth сервиса и собираем
WORKDIR /app/services/auth
RUN go build -o auth-service ./cmd/auth

# Финальный образ
FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/services/auth/auth-service .

EXPOSE 8081 50051

CMD ["./auth-service"]