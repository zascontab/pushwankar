cat > verification_docker.sql << 'EOL'
-- Script para verificar la creación del esquema y tablas

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

-- Verificación de índices
DO $$
DECLARE
    index_count INTEGER;
BEGIN
    SELECT COUNT(*) INTO index_count
    FROM pg_indexes
    WHERE schemaname = 'notification_service';
    
    RAISE NOTICE 'ÍNDICES: Se encontraron % índices en el esquema notification_service', index_count;
END
$$;

-- Resumen final
DO $$
BEGIN
    RAISE NOTICE '----------- RESUMEN -----------';
    RAISE NOTICE 'Dispositivos: Table check - %', (SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'notification_service' AND table_name = 'devices'));
    RAISE NOTICE 'Tokens: Table check - %', (SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'notification_service' AND table_name = 'notification_tokens'));
    RAISE NOTICE 'Notificaciones: Table check - %', (SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'notification_service' AND table_name = 'notifications'));
    RAISE NOTICE 'Registros de entrega: Table check - %', (SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'notification_service' AND table_name = 'delivery_tracking'));
    RAISE NOTICE 'Cola de mensajes: Table check - %', (SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'notification_service' AND table_name = 'message_queue'));
    RAISE NOTICE '------------------------------------------';
END
$$;
EOL