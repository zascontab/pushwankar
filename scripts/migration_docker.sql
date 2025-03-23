cat > migration_docker.sql << 'EOL'
-- Script de migración para crear el esquema y tablas del servicio de notificaciones

-- Crear esquema si no existe
CREATE SCHEMA IF NOT EXISTS notification_service;

-- Tablas requeridas en notification_service
-- 1. Tabla de dispositivos
CREATE TABLE IF NOT EXISTS notification_service.devices (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id INTEGER,
  device_identifier TEXT NOT NULL,
  model TEXT,
  verified BOOLEAN DEFAULT FALSE,
  last_access TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
  last_used TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
  create_time TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
  update_time TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
  delete_time TIMESTAMP WITH TIME ZONE,
  status TEXT DEFAULT 'active'
);

CREATE INDEX IF NOT EXISTS idx_devices_user_id ON notification_service.devices(user_id);
CREATE INDEX IF NOT EXISTS idx_devices_identifier ON notification_service.devices(device_identifier);

-- 2. Tabla de tokens de notificación
CREATE TABLE IF NOT EXISTS notification_service.notification_tokens (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  device_id UUID NOT NULL REFERENCES notification_service.devices(id),
  token TEXT NOT NULL,
  token_type TEXT NOT NULL DEFAULT 'websocket',
  created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
  expires_at TIMESTAMP WITH TIME ZONE,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  is_revoked BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE INDEX IF NOT EXISTS idx_notification_tokens_device_id ON notification_service.notification_tokens(device_id);
CREATE INDEX IF NOT EXISTS idx_notification_tokens_token_type ON notification_service.notification_tokens(token_type);

-- 3. Tabla de notificaciones
CREATE TABLE IF NOT EXISTS notification_service.notifications (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id TEXT NOT NULL,
  title TEXT NOT NULL,
  message TEXT NOT NULL,
  data JSONB,
  notification_type TEXT DEFAULT 'normal',
  sender_id TEXT,
  priority INTEGER DEFAULT 0,
  created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
  expires_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_notifications_user_id ON notification_service.notifications(user_id);
CREATE INDEX IF NOT EXISTS idx_notifications_created_at ON notification_service.notifications(created_at);

-- 4. Tabla de seguimiento de entregas
CREATE TABLE IF NOT EXISTS notification_service.delivery_tracking (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  notification_id UUID NOT NULL REFERENCES notification_service.notifications(id),
  device_id UUID NOT NULL REFERENCES notification_service.devices(id),
  channel TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',
  sent_at TIMESTAMP WITH TIME ZONE,
  delivered_at TIMESTAMP WITH TIME ZONE,
  failed_at TIMESTAMP WITH TIME ZONE,
  retry_count INTEGER DEFAULT 0,
  error_message TEXT,
  created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_delivery_notification_id ON notification_service.delivery_tracking(notification_id);
CREATE INDEX IF NOT EXISTS idx_delivery_device_id ON notification_service.delivery_tracking(device_id);
CREATE INDEX IF NOT EXISTS idx_delivery_status ON notification_service.delivery_tracking(status);

-- 5. Tabla para cola de mensajes
CREATE TABLE IF NOT EXISTS notification_service.message_queue (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  notification_id UUID NOT NULL REFERENCES notification_service.notifications(id),
  device_id UUID NOT NULL REFERENCES notification_service.devices(id),
  payload JSONB NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',
  created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
  next_attempt_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
  retry_count INTEGER DEFAULT 0,
  max_retries INTEGER DEFAULT 5
);

CREATE INDEX IF NOT EXISTS idx_message_queue_status ON notification_service.message_queue(status);
CREATE INDEX IF NOT EXISTS idx_message_queue_next_attempt ON notification_service.message_queue(next_attempt_at);
CREATE INDEX IF NOT EXISTS idx_message_queue_device ON notification_service.message_queue(device_id);

-- Asegurarse de tener la extensión uuid-ossp para generar UUIDs
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Mensaje final
DO $$
BEGIN
    RAISE NOTICE 'Esquema y tablas del servicio de notificaciones creados correctamente.';
END
$$;
EOL