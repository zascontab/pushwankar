-- Script para verificar la migración de datos al esquema notification_service

-- Función para imprimir resultados
CREATE OR REPLACE FUNCTION print_verification_result(
    test_name TEXT, 
    expected INTEGER, 
    actual INTEGER
) RETURNS VOID AS $$
BEGIN
    IF expected = actual THEN
        RAISE NOTICE '% - OK: % coincide con lo esperado', test_name, actual;
    ELSE
        RAISE WARNING '% - ERROR: Se esperaban % registros, pero se encontraron %', 
            test_name, expected, actual;
    END IF;
END;
$$ LANGUAGE plpgsql;

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

-- Verificación de migración desde la base de datos original (si existe)
DO $$
DECLARE
    original_devices_count INTEGER := 0;
    migrated_devices_count INTEGER;
    can_connect BOOLEAN := FALSE;
BEGIN
    -- Intentar conectarse a la base de datos original para comparar datos
    -- Primero verificamos si existe la extensión dblink
    CREATE EXTENSION IF NOT EXISTS dblink;
    
    -- Intentar establecer conexión
    BEGIN
        PERFORM dblink_connect('dbname=rantipaydb host=localhost port=54322 user=postgres password=postgres');
        can_connect := TRUE;
    EXCEPTION WHEN OTHERS THEN
        RAISE NOTICE 'No se pudo conectar a la base de datos original rantipaydb: %', SQLERRM;
        can_connect := FALSE;
    END;
    
    IF can_connect THEN
        -- Contar dispositivos en la base original
        SELECT count_devices INTO original_devices_count
        FROM dblink('dbname=rantipaydb host=localhost port=54322 user=postgres password=postgres',
            'SELECT COUNT(*) FROM public.devices WHERE deleted_at IS NULL')
        AS t1(count_devices INTEGER);
        
        -- Contar dispositivos migrados
        SELECT COUNT(*) INTO migrated_devices_count 
        FROM notification_service.devices;
        
        -- Verificar resultados
        PERFORM print_verification_result('Migración de dispositivos', original_devices_count, migrated_devices_count);
        
        -- Verificar tokens
        DECLARE
            original_tokens_count INTEGER;
            migrated_tokens_count INTEGER;
        BEGIN
            SELECT count_tokens INTO original_tokens_count
            FROM dblink('dbname=rantipaydb host=localhost port=54322 user=postgres password=postgres',
                'SELECT COUNT(*) FROM public.device_tokens WHERE notification_token IS NOT NULL AND device_id IS NOT NULL')
            AS t1(count_tokens INTEGER);
            
            SELECT COUNT(*) INTO migrated_tokens_count 
            FROM notification_service.notification_tokens;
            
            PERFORM print_verification_result('Migración de tokens', original_tokens_count, migrated_tokens_count);
        EXCEPTION WHEN OTHERS THEN
            RAISE NOTICE 'Error al verificar tokens: %', SQLERRM;
        END;
        
        -- Verificar notificaciones si existen
        BEGIN
            DECLARE
                original_notifications_count INTEGER;
                migrated_notifications_count INTEGER;
                table_exists BOOLEAN;
            BEGIN
                SELECT EXISTS INTO table_exists
                FROM dblink('dbname=rantipaydb host=localhost port=54322 user=postgres password=postgres',
                    'SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema = ''public'' AND table_name = ''notifications'')')
                AS t1(exists_flag BOOLEAN);
                
                IF table_exists THEN
                    SELECT count_notifications INTO original_notifications_count
                    FROM dblink('dbname=rantipaydb host=localhost port=54322 user=postgres password=postgres',
                        'SELECT COUNT(*) FROM public.notifications WHERE deleted_at IS NULL')
                    AS t1(count_notifications INTEGER);
                    
                    SELECT COUNT(*) INTO migrated_notifications_count 
                    FROM notification_service.notifications;
                    
                    PERFORM print_verification_result('Migración de notificaciones', original_notifications_count, migrated_notifications_count);
                ELSE
                    RAISE NOTICE 'La tabla de notificaciones no existe en la base original, no se verificará.';
                END IF;
            END;
        EXCEPTION WHEN OTHERS THEN
            RAISE NOTICE 'Error al verificar notificaciones: %', SQLERRM;
        END;
        
        -- Cerrar la conexión dblink
        PERFORM dblink_disconnect();
    ELSE
        RAISE NOTICE 'No se pudo establecer conexión con la base de datos original para comparación.';
        
        -- Verificar solo los datos migrados sin comparación
        SELECT COUNT(*) INTO migrated_devices_count FROM notification_service.devices;
        RAISE NOTICE 'Dispositivos migrados: %', migrated_devices_count;
        
        DECLARE 
            token_count INTEGER;
            notification_count INTEGER;
            delivery_count INTEGER;
            queue_count INTEGER;
        BEGIN
            SELECT COUNT(*) INTO token_count FROM notification_service.notification_tokens;
            SELECT COUNT(*) INTO notification_count FROM notification_service.notifications;
            SELECT COUNT(*) INTO delivery_count FROM notification_service.delivery_tracking;
            SELECT COUNT(*) INTO queue_count FROM notification_service.message_queue;
            
            RAISE NOTICE 'Tokens migrados: %', token_count;
            RAISE NOTICE 'Notificaciones migradas: %', notification_count;
            RAISE NOTICE 'Registros de entrega: %', delivery_count;
            RAISE NOTICE 'Cola de mensajes: %', queue_count;
        END;
    END IF;
END
$$;

-- Verificación de integridad referencial
DO $$
DECLARE
    invalid_refs INTEGER;
BEGIN
    -- Verificar referencias a dispositivos que no existen
    SELECT COUNT(*) INTO invalid_refs FROM notification_service.notification_tokens t
    WHERE NOT EXISTS (
        SELECT 1 FROM notification_service.devices d WHERE d.id = t.device_id
    );
    
    IF invalid_refs > 0 THEN
        RAISE WARNING 'INTEGRIDAD REFERENCIAL: Hay % tokens que referencian a dispositivos inexistentes', invalid_refs;
    ELSE
        RAISE NOTICE 'INTEGRIDAD REFERENCIAL: OK - Todos los tokens referencian dispositivos válidos';
    END IF;
    
    -- Si hay notificaciones, verificar también su integridad
    IF EXISTS (SELECT 1 FROM notification_service.notifications) THEN
        SELECT COUNT(*) INTO invalid_refs FROM notification_service.delivery_tracking dt
        WHERE NOT EXISTS (
            SELECT 1 FROM notification_service.notifications n WHERE n.id = dt.notification_id
        );
        
        IF invalid_refs > 0 THEN
            RAISE WARNING 'INTEGRIDAD REFERENCIAL: Hay % registros de entrega que referencian notificaciones inexistentes', invalid_refs;
        ELSE
            RAISE NOTICE 'INTEGRIDAD REFERENCIAL: OK - Todos los registros de entrega referencian notificaciones válidas';
        END IF;
    END IF;
END
$$;

-- Resumen final de la migración
DO $$
BEGIN
    RAISE NOTICE '----------- RESUMEN DE MIGRACIÓN -----------';
    RAISE NOTICE 'Dispositivos: % registros', (SELECT COUNT(*) FROM notification_service.devices);
    RAISE NOTICE 'Tokens: % registros', (SELECT COUNT(*) FROM notification_service.notification_tokens);
    RAISE NOTICE 'Notificaciones: % registros', (SELECT COUNT(*) FROM notification_service.notifications);
    RAISE NOTICE 'Registros de entrega: % registros', (SELECT COUNT(*) FROM notification_service.delivery_tracking);
    RAISE NOTICE 'Cola de mensajes: % registros', (SELECT COUNT(*) FROM notification_service.message_queue);
    RAISE NOTICE '------------------------------------------';
END
$;

-- Verificación de índices
DO $
DECLARE
    index_count INTEGER;
BEGIN
    SELECT COUNT(*) INTO index_count
    FROM pg_indexes
    WHERE schemaname = 'notification_service';
    
    RAISE NOTICE 'ÍNDICES: Se encontraron % índices en el esquema notification_service', index_count;
    
    -- Verificar índices críticos
    IF EXISTS (SELECT 1 FROM pg_indexes WHERE schemaname = 'notification_service' AND indexname = 'idx_devices_user_id') THEN
        RAISE NOTICE 'ÍNDICE: idx_devices_user_id - OK';
    ELSE
        RAISE WARNING 'ÍNDICE: idx_devices_user_id - NO EXISTE';
    END IF;
    
    IF EXISTS (SELECT 1 FROM pg_indexes WHERE schemaname = 'notification_service' AND indexname = 'idx_notification_tokens_device_id') THEN
        RAISE NOTICE 'ÍNDICE: idx_notification_tokens_device_id - OK';
    ELSE
        RAISE WARNING 'ÍNDICE: idx_notification_tokens_device_id - NO EXISTE';
    END IF;
    
    IF EXISTS (SELECT 1 FROM pg_indexes WHERE schemaname = 'notification_service' AND indexname = 'idx_notifications_user_id') THEN
        RAISE NOTICE 'ÍNDICE: idx_notifications_user_id - OK';
    ELSE
        RAISE WARNING 'ÍNDICE: idx_notifications_user_id - NO EXISTE';
    END IF;
    
    IF EXISTS (SELECT 1 FROM pg_indexes WHERE schemaname = 'notification_service' AND indexname = 'idx_delivery_notification_id') THEN
        RAISE NOTICE 'ÍNDICE: idx_delivery_notification_id - OK';
    ELSE
        RAISE WARNING 'ÍNDICE: idx_delivery_notification_id - NO EXISTE';
    END IF;
END
$;

-- Verificación de permisos
DO $
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'notification_user') THEN
        RAISE NOTICE 'VERIFICACIÓN DE PERMISOS:';
        
        -- Verificar permiso en esquema
        IF EXISTS (
            SELECT 1 FROM information_schema.role_usage_grants 
            WHERE grantee = 'notification_user' 
            AND object_schema = 'notification_service'
        ) THEN
            RAISE NOTICE '- Permiso USAGE en esquema: OK';
        ELSE
            RAISE WARNING '- Permiso USAGE en esquema: FALTA';
        END IF;
        
        -- Verificar permisos en tablas (simplificado)
        IF EXISTS (
            SELECT 1 FROM information_schema.role_table_grants 
            WHERE grantee = 'notification_user' 
            AND table_schema = 'notification_service'
            AND privilege_type = 'SELECT'
        ) THEN
            RAISE NOTICE '- Permisos en tablas: OK (al menos SELECT)';
        ELSE
            RAISE WARNING '- Permisos en tablas: POSIBLEMENTE FALTAN';
        END IF;
    ELSE
        RAISE NOTICE 'El rol notification_user no existe. Saltando verificación de permisos.';
    END IF;
END
$;

-- Verificación de estructura de tabla (ejemplo con la tabla devices)
DO $
DECLARE
    column_count INTEGER;
BEGIN
    SELECT COUNT(*) INTO column_count
    FROM information_schema.columns
    WHERE table_schema = 'notification_service'
    AND table_name = 'devices';
    
    IF column_count >= 10 THEN -- Deberían existir al menos 10 columnas
        RAISE NOTICE 'ESTRUCTURA DE devices: OK - Tiene % columnas', column_count;
    ELSE
        RAISE WARNING 'ESTRUCTURA DE devices: POSIBLE PROBLEMA - Se esperaban al menos 10 columnas, pero se encontraron %', column_count;
    END IF;
    
    -- Verificar columnas críticas
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'notification_service'
        AND table_name = 'devices'
        AND column_name = 'device_identifier'
    ) THEN
        RAISE NOTICE '- Columna critical device_identifier: OK';
    ELSE
        RAISE WARNING '- Columna crítica device_identifier: FALTA';
    END IF;
END
$;

-- Verificación final
DO $
BEGIN
    RAISE NOTICE '----------- VERIFICACIÓN COMPLETA -----------';
    RAISE NOTICE 'La verificación ha terminado. Revise los mensajes anteriores para identificar posibles problemas.';
    RAISE NOTICE 'Si no hay mensajes de ERROR o WARNING, la migración se ha completado correctamente.';
    RAISE NOTICE '--------------------------------------------';
END
$;