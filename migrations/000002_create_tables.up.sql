-- Tabla para almacenar tokens de notificación
CREATE TABLE notification_service.notification_tokens (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  device_id UUID NOT NULL,
  token TEXT NOT NULL,
  token_type TEXT NOT NULL DEFAULT 'websocket',
  created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
  expires_at TIMESTAMP WITH TIME ZONE,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  is_revoked BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE INDEX idx_notification_tokens_device_id ON notification_service.notification_tokens(device_id);
CREATE INDEX idx_notification_tokens_token_type ON notification_service.notification_tokens(token_type);

-- Tabla para almacenar los dispositivos (referencia a los del monolito)
CREATE TABLE notification_service.devices (
  id UUID PRIMARY KEY,
  user_id INTEGER,
  device_identifier TEXT NOT NULL,
  model TEXT,
  verified BOOLEAN DEFAULT FALSE,
  last_access TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
  created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_devices_user_id ON notification_service.devices(user_id);
CREATE INDEX idx_devices_identifier ON notification_service.devices(device_identifier);

-- Tabla para almacenar notificaciones
CREATE TABLE notification_service.notifications (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
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

CREATE INDEX idx_notifications_user_id ON notification_service.notifications(user_id);
CREATE INDEX idx_notifications_created_at ON notification_service.notifications(created_at);

-- Tabla para rastrear el estado de entrega
CREATE TABLE notification_service.delivery_tracking (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  notification_id UUID NOT NULL REFERENCES notification_service.notifications(id),
  device_id UUID NOT NULL,
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

CREATE INDEX idx_delivery_notification_id ON notification_service.delivery_tracking(notification_id);
CREATE INDEX idx_delivery_device_id ON notification_service.delivery_tracking(device_id);
CREATE INDEX idx_delivery_status ON notification_service.delivery_tracking(status);

-- Tabla para la cola de mensajes
CREATE TABLE notification_service.message_queue (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  notification_id UUID NOT NULL REFERENCES notification_service.notifications(id),
  device_id UUID NOT NULL,
  payload JSONB NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',
  created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
  next_attempt_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
  retry_count INTEGER DEFAULT 0,
  max_retries INTEGER DEFAULT 5
);

CREATE INDEX idx_message_queue_status ON notification_service.message_queue(status);
CREATE INDEX idx_message_queue_next_attempt ON notification_service.message_queue(next_attempt_at);
CREATE INDEX idx_message_queue_device ON notification_service.message_queue(device_id);