-- Script para verificar la creación del esquema y tablas en PostgreSQL 15

-- Verificación de existencia del esquema
DO $$
BEGIN
    IF EXISTS (
        SELECT FROM information_schema.schemata 
        WHERE schema_name = 'notification_service'
    ) THEN
        RAISE NOTICE 'ESQUEMA: OK - El esquema notification_service existe';
    ELSE
        RAISE WARNING 'ESQUEMA: ERROR - El esquema notification_service NO existe';
        RETURN;
    END IF;
END
$$;

-- Verificación de tablas
DO $$
DECLARE
    tables_count INTEGER;
BEGIN
    SELECT COUNT(*) INTO tables_count
    FROM information_schema.tables
    WHERE table_schema = 'notification_service'
    AND table_type = 'BASE TABLE';
    
    IF tables_count >= 5 THEN -- Deberían existir al menos 5 tablas
        RAISE NOTICE 'TABLAS: OK - Se encontraron % tablas en el esquema', tables_count;
    ELSE
        RAISE WARNING 'TABLAS: ERROR - Se esperaban al menos 5 tablas, pero se encontraron %', tables_count;
    END IF;
END
$$;

-- Verificación de tablas específicas
DO $$
BEGIN
    -- Dispositivos
    IF EXISTS (
        SELECT FROM information_schema.tables
        WHERE table_schema = 'notification_service' AND table_name = 'devices'
    ) THEN
        RAISE NOTICE 'TABLA: devices - OK';
    ELSE
        RAISE WARNING 'TABLA: devices - NO EXISTE';
    END IF;
    
    -- Tokens
    IF EXISTS (
        SELECT FROM information_schema.tables
        WHERE table_schema = 'notification_service' AND table_name = 'notification_tokens'
    ) THEN
        RAISE NOTICE 'TABLA: notification_tokens - OK';
    ELSE
        RAISE WARNING 'TABLA: notification_tokens - NO EXISTE';
    END IF;
    
    -- Notificaciones
    IF EXISTS (
        SELECT FROM information_schema.tables
        WHERE table_schema = 'notification_service' AND table_name = 'notifications'
    ) THEN
        RAISE NOTICE 'TABLA: notifications - OK';
    ELSE
        RAISE WARNING 'TABLA: notifications - NO EXISTE';
    END IF;
    
    -- Seguimiento de entregas
    IF EXISTS (
        SELECT FROM information_schema.tables
        WHERE table_schema = 'notification_service' AND table_name = 'delivery_tracking'
    ) THEN
        RAISE NOTICE 'TABLA: delivery_tracking - OK';
    ELSE
        RAISE WARNING 'TABLA: delivery_tracking - NO EXISTE';
    END IF;
    
    -- Cola de mensajes
    IF EXISTS (
        SELECT FROM information_schema.tables
        WHERE table_schema = 'notification_service' AND table_name = 'message_queue'
    ) THEN
        RAISE NOTICE 'TABLA: message_queue - OK';
    ELSE
        RAISE WARNING 'TABLA: message_queue - NO EXISTE';
    END IF;
END
$$;

-- Verificación de columnas críticas (solo para una tabla como ejemplo)
DO $$
BEGIN
    -- Verificar columnas de devices
    IF EXISTS (
        SELECT FROM information_schema.columns
        WHERE table_schema = 'notification_service' AND table_name = 'devices' AND column_name = 'device_identifier'
    ) THEN
        RAISE NOTICE 'COLUMNA: devices.device_identifier - OK';
    ELSE
        RAISE WARNING 'COLUMNA: devices.device_identifier - NO EXISTE';
    END IF;
    
    -- Verificar columnas de notifications
    IF EXISTS (
        SELECT FROM information_schema.columns
        WHERE table_schema = 'notification_service' AND table_name = 'notifications' AND column_name = 'data'
    ) THEN
        RAISE NOTICE 'COLUMNA: notifications.data - OK';
    ELSE
        RAISE WARNING 'COLUMNA: notifications.data - NO EXISTE';
    END IF;
END
$$;

-- Resumen final
DO $$
BEGIN
    RAISE NOTICE '----------- RESUMEN -----------';
    RAISE NOTICE 'La verificación ha terminado. Si no hay mensajes de ERROR o WARNING, la migración se ha completado correctamente.';
    RAISE NOTICE '------------------------------------------';
END
$$;
