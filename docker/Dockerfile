# Etapa de construcción
FROM golang:1.19-alpine AS builder

# Instalar certificados y dependencias
RUN apk --no-cache add ca-certificates tzdata git

# Configurar directorio de trabajo
WORKDIR /app

# Copiar módulos Go
COPY go.mod go.sum ./
RUN go mod download

# Copiar el código fuente
COPY . .

# Compilar la aplicación
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o notification-service ./cmd/server

# Etapa final
FROM alpine:3.15

# Instalar certificados y zona horaria
RUN apk --no-cache add ca-certificates tzdata

# Copiar el ejecutable desde la etapa de construcción
COPY --from=builder /app/notification-service /usr/local/bin/notification-service

# Copiar archivos de migración
COPY --from=builder /app/migrations /migrations

# Exponer puertos
EXPOSE 8080
EXPOSE 9090

# Definir comando de inicio
CMD ["notification-service"]