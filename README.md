# notification
# API de Servicio de Notificaciones

Esta documentación describe las APIs disponibles en el Servicio de Notificaciones de Rantipay.

## Índice

1. [Introducción](#introducción)
2. [Base URL](#base-url)
3. [Autenticación](#autenticación)
4. [Formato de Respuesta](#formato-de-respuesta)
5. [API HTTP](#api-http)
   - [Notificaciones](#notificaciones)
   - [Dispositivos](#dispositivos)
   - [Estado](#estado)
6. [API gRPC](#api-grpc)
7. [WebSockets](#websockets)
8. [Códigos de Error](#códigos-de-error)
9. [Ejemplos de Uso](#ejemplos-de-uso)

## Introducción

El Servicio de Notificaciones proporciona funcionalidades para:

- Enviar notificaciones push a dispositivos y usuarios
- Gestionar dispositivos y tokens de notificación
- Verificar estado de entrega de notificaciones
- Conexiones WebSocket para notificaciones en tiempo real

## Base URL

**Entorno de Producción**
```
https://notifications-api.rantipay.com/api/v1
```

**Entorno de Staging**
```
https://notifications-api.staging.rantipay.com/api/v1
```

**WebSocket URL**
```
wss://notifications-api.rantipay.com/ws
```

## Autenticación

### API HTTP

La API HTTP utiliza tokens de autenticación JWT en la cabecera `Authorization`. Cada solicitud debe incluir esta cabecera en el siguiente formato:

```
Authorization: Bearer <token>
```

### WebSocket

La conexión WebSocket requiere un token JWT como parámetro de consulta:

```
wss://notifications-api.rantipay.com/ws?token=<token>
```

## Formato de Respuesta

Las respuestas de la API HTTP utilizan el formato JSON. Todas las respuestas incluyen un campo `success` que indica si la operación fue exitosa, y en caso de error, un campo `error` con el mensaje de error.

Ejemplo de respuesta exitosa:
```json
{
  "success": true,
  "data": {
    "notification_id": "6a7b8c9d-1e2f-3a4b-5c6d-7e8f9a0b1c2d"
  }
}
```

Ejemplo de respuesta de error:
```json
{
  "success": false,
  "error": "Invalid token"
}
```

## API HTTP

### Notificaciones

#### Enviar Notificación

**POST /notifications/send**

Envía una notificación a un usuario específico.

**Cuerpo de la Solicitud**

```json
{
  "user_id": "12345",
  "title": "Nuevo mensaje",
  "message": "Has recibido un nuevo mensaje",
  "data": {
    "sender_id": "67890",
    "message_id": "abc123",
    "custom_field": "valor personalizado"
  },
  "notification_type": "message" // Opcional: normal, urgent, system, message
}
```

**Respuesta**

```json
{
  "success": true,
  "notification_id": "6a7b8c9d-1e2f-3a4b-5c6d-7e8f9a0b1c2d"
}
```

#### Obtener Estado de Notificación

**GET /notifications/{id}**

Obtiene el estado de entrega de una notificación específica.

**Respuesta**

```json
{
  "id": "6a7b8c9d-1e2f-3a4b-5c6d-7e8f9a0b1c2d",
  "user_id": "12345",
  "title": "Nuevo mensaje",
  "message": "Has recibido un nuevo mensaje",
  "data": {
    "sender_id": "67890",
    "message_id": "abc123"
  },
  "type": "message",
  "created_at": 1647532800,
  "deliveries": [
    {
      "device_id": "d1e2f3a4-b5c6-7d8e-9f0a-1b2c3d4e5f6a",
      "status": "delivered",
      "sent_at": 1647532805,
      "delivered_at": 1647532810,
      "retry_count": 0
    },
    {
      "device_id": "a1b2c3d4-e5f6-7a8b-9c0d-1e2f3a4b5c6d",
      "status": "failed",
      "sent_at": 1647532805,
      "failed_at": 1647532815,
      "retry_count": 3,
      "error_message": "Device not connected"
    }
  ]
}
```

#### Obtener Notificaciones de Usuario

**GET /users/{user_id}/notifications**

Obtiene las notificaciones de un usuario específico.

**Parámetros de Consulta**

- `limit` (opcional): Número máximo de notificaciones a devolver. Valor predeterminado: 20.
- `offset` (opcional): Desplazamiento para paginación. Valor predeterminado: 0.

**Respuesta**

```json
{
  "notifications": [
    {
      "id": "6a7b8c9d-1e2f-3a4b-5c6d-7e8f9a0b1c2d",
      "title": "Nuevo mensaje",
      "message": "Has recibido un nuevo mensaje",
      "data": {
        "sender_id": "67890",
        "message_id": "abc123"
      },
      "type": "message",
      "created_at": 1647532800
    },
    {
      "id": "f6a7b8c9-d1e2-f3a4-b5c6-d7e8f9a0b1c2",
      "title": "Actualización disponible",
      "message": "Hay una nueva versión disponible",
      "data": {
        "version": "1.2.0"
      },
      "type": "system",
      "created_at": 1647446400
    }
  ],
  "unread_count": 5,
  "total": 2,
  "limit": 20,
  "offset": 0
}
```

#### Confirmar Entrega

**POST /notifications/confirm**

Confirma la entrega de una notificación a un dispositivo específico.

**Cuerpo de la Solicitud**

```json
{
  "notification_id": "6a7b8c9d-1e2f-3a4b-5c6d-7e8f9a0b1c2d",
  "device_id": "d1e2f3a4-b5c6-7d8e-9f0a-1b2c3d4e5f6a"
}
```

**Respuesta**

```json
{
  "status": "success"
}
```

### Dispositivos

#### Registrar Dispositivo

**POST /devices/register**

Registra un nuevo dispositivo para recibir notificaciones.

**Cuerpo de la Solicitud**

```json
{
  "device_identifier": "unique-device-id-123",
  "user_id": "12345", // Opcional
  "model": "iPhone 13" // Opcional
}
```

**Respuesta**

```json
{
  "device_id": "d1e2f3a4-b5c6-7d8e-9f0a-1b2c3d4e5f6a",
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "is_verified": false
}
```

#### Obtener Dispositivo

**GET /devices/{id}**

Obtiene información sobre un dispositivo específico.

**Respuesta**

```json
{
  "id": "d1e2f3a4-b5c6-7d8e-9f0a-1b2c3d4e5f6a",
  "user_id": "12345",
  "device_identifier": "unique-device-id-123",
  "model": "iPhone 13",
  "verified": true,
  "last_access": "2023-03-15T14:30:00Z",
  "last_used": "2023-03-15T14:30:00Z",
  "create_time": "2023-03-10T09:15:00Z",
  "update_time": "2023-03-15T14:30:00Z",
  "status": "active"
}
```

#### Vincular Dispositivo a Usuario

**POST /devices/link**

Vincula un dispositivo a un usuario específico.

**Cuerpo de la Solicitud**

```json
{
  "device_id": "d1e2f3a4-b5c6-7d8e-9f0a-1b2c3d4e5f6a",
  "user_id": "12345",
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." // Token temporal
}
```

**Respuesta**

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." // Nuevo token permanente
}
```

#### Actualizar Token

**POST /devices/token**

Actualiza un token de notificación para un dispositivo específico.

**Cuerpo de la Solicitud**

```json
{
  "device_id": "d1e2f3a4-b5c6-7d8e-9f0a-1b2c3d4e5f6a",
  "token": "fcm-token-123456",
  "token_type": "fcm" // websocket, apns, fcm
}
```

**Respuesta**

```json
{
  "status": "success"
}
```

### Estado

#### Verificar Estado del Servicio

**GET /health**

Verifica el estado del servicio.

**Respuesta**

```json
{
  "status": "ok",
  "uptime": "10h30m15s",
  "timestamp": "2023-03-18T12:30:45Z",
  "service": "notification-service",
  "version": "1.0.0"
}
```

## API gRPC

El servicio también proporciona una API gRPC para comunicación interna entre servicios. Los contratos están definidos en los archivos `.proto` incluidos en el repositorio.

Servicios disponibles:

1. `NotificationService`: Para enviar y gestionar notificaciones
2. `BusinessService`: Para validar usuarios y obtener información de dispositivos

## WebSockets

### Conexión

Para establecer una conexión WebSocket, conectarse a:

```
wss://notifications-api.rantipay.com/ws?token=<token>
```

Donde `<token>` es un token JWT válido obtenido al registrar el dispositivo o al vincularlo a un usuario.

### Mensajes

Los mensajes WebSocket utilizan el formato JSON y deben incluir un campo `type` que indica el tipo de mensaje.

#### Mensajes del Cliente al Servidor

**Ping**

```json
{
  "type": "ping"
}
```

**Refresco de Token**

```json
{
  "type": "token_refresh",
  "payload": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  }
}
```

**Confirmación de Entrega**

```json
{
  "type": "ack",
  "payload": {
    "notification_id": "6a7b8c9d-1e2f-3a4b-5c6d-7e8f9a0b1c2d"
  }
}
```

#### Mensajes del Servidor al Cliente

**Pong**

```json
{
  "type": "pong",
  "timestamp": "2023-03-18T12:30:45Z"
}
```

**Notificación**

```json
{
  "type": "notification",
  "notification_id": "6a7b8c9d-1e2f-3a4b-5c6d-7e8f9a0b1c2d",
  "title": "Nuevo mensaje",
  "message": "Has recibido un nuevo mensaje",
  "data": {
    "sender_id": "67890",
    "message_id": "abc123"
  },
  "timestamp": 1647532800
}
```

**Respuesta de Refresco de Token**

```json
{
  "type": "token_refresh_response",
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "success": true
}
```

## Códigos de Error

| Código HTTP | Descripción |
|-------------|-------------|
| 200 | OK - La solicitud se completó correctamente |
| 400 | Bad Request - La solicitud tiene un formato incorrecto o faltan parámetros |
| 401 | Unauthorized - No hay token de autenticación o es inválido |
| 403 | Forbidden - No tiene permisos para acceder al recurso |
| 404 | Not Found - El recurso no existe |
| 429 | Too Many Requests - Se ha excedido el límite de solicitudes |
| 500 | Internal Server Error - Error interno del servidor |
| 503 | Service Unavailable - El servicio no está disponible temporalmente |

## Ejemplos de Uso

### Enviar una Notificación

**Solicitud**

```bash
curl -X POST https://notifications-api.rantipay.com/api/v1/notifications/send \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "12345",
    "title": "Recordatorio de pago",
    "message": "Tienes un pago pendiente para mañana",
    "data": {
      "payment_id": "pay_123456",
      "amount": "150.00",
      "currency": "USD"
    },
    "notification_type": "normal"
  }'
```

**Respuesta**

```json
{
  "success": true,
  "notification_id": "6a7b8c9d-1e2f-3a4b-5c6d-7e8f9a0b1c2d"
}
```

### Registrar un Dispositivo

**Solicitud**

```bash
curl -X POST https://notifications-api.rantipay.com/api/v1/devices/register \
  -H "Content-Type: application/json" \
  -d '{
    "device_identifier": "device-id-abc123",
    "user_id": "12345",
    "model": "Samsung Galaxy S21"
  }'
```

**Respuesta**

```json
{
  "device_id": "d1e2f3a4-b5c6-7d8e-9f0a-1b2c3d4e5f6a",
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "is_verified": false
}
```

### Actualizar un Token FCM

**Solicitud**

```bash
curl -X POST https://notifications-api.rantipay.com/api/v1/devices/token \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." \
  -H "Content-Type: application/json" \
  -d '{
    "device_id": "d1e2f3a4-b5c6-7d8e-9f0a-1b2c3d4e5f6a",
    "token": "fMEQzpRfTbugfGyOleFbiP:APA91bHun4MxP5MYwOnZ...",
    "token_type": "fcm"
  }'
```

**Respuesta**

```json
{
  "status": "success"
}
```

### Uso de WebSocket con JavaScript

```javascript
// Establecer conexión WebSocket
const token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...";
const socket = new WebSocket(`wss://notifications-api.rantipay.com/ws?token=${token}`);

// Escuchar eventos
socket.onopen = () => {
  console.log('Conexión establecida');
  
  // Enviar ping para mantener la conexión
  setInterval(() => {
    socket.send(JSON.stringify({ type: 'ping' }));
  }, 30000);
};

socket.onmessage = (event) => {
  const data = JSON.parse(event.data);
  
  switch(data.type) {
    case 'notification':
      console.log('Nueva notificación:', data);
      // Mostrar notificación al usuario
      displayNotification(data);
      
      // Confirmar recepción
      socket.send(JSON.stringify({
        type: 'ack',
        payload: {
          notification_id: data.notification_id
        }
      }));
      break;
      
    case 'pong':
      console.log('Pong recibido:', data.timestamp);
      break;
      
    case 'token_refresh_response':
      console.log('Token actualizado:', data.success);
      if (data.success) {
        // Guardar el nuevo token
        saveToken(data.token);
      }
      break;
  }
};

socket.onclose = (event) => {
  console.log('Conexión cerrada:', event.code, event.reason);
  // Intentar reconectar después de un tiempo
  setTimeout(reconnect, 5000);
};

socket.onerror = (error) => {
  console.error('Error en la conexión WebSocket:', error);
};

// Función para manejar el token expirado
function handleExpiredToken() {
  // Solicitar nuevo token
  socket.send(JSON.stringify({
    type: 'token_refresh',
    payload: {
      token: getCurrentToken()
    }
  }));
}
```

## Límites y Cuotas

- Máximo de 100 solicitudes por minuto por API Key
- Máximo de 1,000 conexiones WebSocket concurrentes por usuario
- Tamaño máximo de payload de notificación: 4KB
- Retención de historial de notificaciones: 30 días

## Mejores Prácticas

1. **Manejo de Errores**
   - Implementar reintentos con backoff exponencial para errores temporales
   - Monitorear tasas de error para detectar problemas

2. **Conexiones WebSocket**
   - Implementar reconexión automática con backoff
   - Enviar pings regularmente para mantener la conexión activa

3. **Tokens**
   - Almacenar tokens de forma segura
   - Implementar actualización automática de tokens expirados

4. **Optimización de Payload**
   - Mantener el tamaño de los datos adicionales lo más pequeño posible
   - Utilizar campos específicos en lugar de datos genéricos cuando sea posible

5. **Prioridad de Notificaciones**
   - Utilizar el tipo "urgent" solo para notificaciones críticas
   - Considerar la zona horaria del usuario al enviar notificaciones generales