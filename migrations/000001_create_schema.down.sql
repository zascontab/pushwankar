-- Crear el esquema para el servicio de notificaciones
CREATE SCHEMA IF NOT EXISTS notification_service;

-- Establecer permisos adecuados
GRANT USAGE ON SCHEMA notification_service TO notification_user;
ALTER DEFAULT PRIVILEGES IN SCHEMA notification_service GRANT ALL ON TABLES TO notification_user;
ALTER DEFAULT PRIVILEGES IN SCHEMA notification_service GRANT USAGE, SELECT ON SEQUENCES TO notification_user;

-- Asegurarse de que tenemos la extensión de UUID
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";