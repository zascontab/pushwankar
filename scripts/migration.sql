-- Script de migración de datos para el servicio de notificaciones
-- Este script migra datos del esquema principal del monolito al nuevo esquema de notificaciones

-- Crear esquema si no existe
CREATE SCHEMA IF NOT EXISTS notification_service;

-- Verificar si existen tablas en notification_service para evitar migración duplicada
DO $$
BEGIN
    IF EXISTS (
        SELECT FROM information_schema.tables 
        WHERE table_schema = 'notification_service' AND table_name = 'delivery_tracking'
    ) THEN
        RAISE NOTICE 'Las tablas ya existen en el esquema notification_service. Verificar si se requiere migración.';
        RETURN;
    END IF;
END
$$;

-- Tablas requeridas en notification_service
-- 1. Tabla de dispositivos
CREATE TABLE IF NOT EXISTS notification_service.devices (
  id UUID PRIMARY KEY,
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

-- Bloque para verificar si la tabla devices existe en el esquema public de la base de datos fuente
DO $$
BEGIN
    -- Intentamos conectarnos primero a la base de datos original (rantipaydb) para migrar datos
    -- Esto asume que tenemos los permisos para conectarnos a otra base de datos
    IF EXISTS (
        SELECT 1 FROM pg_database WHERE datname='rantipaydb'
    ) THEN
        -- Migración de datos: Dispositivos
        -- Nota: Esto requiere una conexión dblink a la base de datos fuente
        CREATE EXTENSION IF NOT EXISTS dblink;
        
        -- Migración de Dispositivos
        INSERT INTO notification_service.devices (
            id, user_id, device_identifier, model, verified, 
            last_access, last_used, create_time, update_time, status
        )
        SELECT 
            d.id, d.user_id, d.device_identifier, d.model, true as verified,
            d.last_access, d.last_access as last_used, d.created_at as create_time, 
            d.updated_at as update_time, 'active' as status
        FROM 
            dblink('dbname=rantipaydb host=localhost port=54322 user=postgres password=postgres',
                'SELECT id, user_id, device_identifier, model, last_access, created_at, updated_at FROM public.devices WHERE deleted_at IS NULL')
            AS d(id uuid, user_id integer, device_identifier text, model text, last_access timestamp with time zone, 
                created_at timestamp with time zone, updated_at timestamp with time zone)
        ON CONFLICT (id) DO NOTHING;
        
        -- Migración de Tokens
        INSERT INTO notification_service.notification_tokens (
            device_id, token, token_type, created_at, updated_at, 
            expires_at, is_active, is_revoked
        )
        SELECT 
            dt.device_id, dt.notification_token as token, 
            -- Determinar tipo de token (ajusta según tu implementación actual)
            COALESCE(dt.token_type, 'websocket') as token_type,
            dt.created_at, dt.updated_at,
            -- Establecer fecha de expiración a 30 días desde la fecha de actualización
            dt.updated_at + INTERVAL '30 days' as expires_at,
            true as is_active, false as is_revoked
        FROM 
            dblink('dbname=rantipaydb host=localhost port=54322 user=postgres password=postgres',
                'SELECT device_id, notification_token, token_type, created_at, updated_at FROM public.device_tokens WHERE notification_token IS NOT NULL AND device_id IS NOT NULL')
            AS dt(device_id uuid, notification_token text, token_type text, created_at timestamp with time zone, updated_at timestamp with time zone)
        ON CONFLICT (id) DO NOTHING;
        
        -- Opcional: Migrar notificaciones si existen
        IF EXISTS (
            SELECT 1 FROM dblink('dbname=rantipaydb host=localhost port=54322 user=postgres password=postgres',
                'SELECT 1 FROM information_schema.tables WHERE table_schema = ''public'' AND table_name = ''notifications'' LIMIT 1')
            AS exists_check(exists_flag integer)
        ) THEN
            -- Migrar notificaciones existentes
            INSERT INTO notification_service.notifications (
                id, user_id, title, message, data, notification_type,
                sender_id, created_at
            )
            SELECT 
                n.id, n.user_id::text, n.title, n.body as message, 
                n.data::jsonb, n.type as notification_type,
                n.sender_id, n.created_at
            FROM 
                dblink('dbname=rantipaydb host=localhost port=54322 user=postgres password=postgres',
                    'SELECT id, user_id, title, body, data, type, sender_id, created_at FROM public.notifications WHERE deleted_at IS NULL')
                AS n(id uuid, user_id integer, title text, body text, data text, type text, sender_id text, created_at timestamp with time zone)
            ON CONFLICT (id) DO NOTHING;
        END IF;
        
        RAISE NOTICE 'Migración de datos completada desde rantipaydb.';
    ELSE
        RAISE NOTICE 'La base de datos rantipaydb no está disponible para migración. Se crearán tablas vacías.';
    END IF;
END
$$;

-- Otorgar permisos
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_roles WHERE rolname = 'notification_user'
    ) THEN
        EXECUTE 'GRANT USAGE ON SCHEMA notification_service TO notification_user';
        EXECUTE 'GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA notification_service TO notification_user';
        EXECUTE 'GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA notification_service TO notification_user';
    ELSE
        RAISE NOTICE 'El rol notification_user no existe. Saltando la asignación de permisos.';
    END IF;
END
$$;

-- Mensaje final
DO $$
BEGIN
    RAISE NOTICE 'Esquema y tablas del servicio de notificaciones creados correctamente.';
END
$$;