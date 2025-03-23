#!/bin/bash

# Script para automatizar la migración de datos del monolito al servicio de notificaciones
# Ejecutar desde un entorno seguro con acceso a las bases de datos

set -e  # Salir inmediatamente si hay un error

# Variables de configuración ajustadas según el archivo .env proporcionado
SOURCE_DB_HOST=${SOURCE_DB_HOST:-"localhost"}
SOURCE_DB_PORT=${SOURCE_DB_PORT:-"54322"}  # Puerto ajustado a 54322
SOURCE_DB_NAME=${SOURCE_DB_NAME:-"rantipaydb"}
SOURCE_DB_USER=${SOURCE_DB_USER:-"postgres"}
SOURCE_DB_PASSWORD=${SOURCE_DB_PASSWORD:-"postgres"}

# Base de datos de destino (puede ser la misma)
TARGET_DB_HOST=${TARGET_DB_HOST:-"$SOURCE_DB_HOST"}
TARGET_DB_PORT=${TARGET_DB_PORT:-"$SOURCE_DB_PORT"}
TARGET_DB_NAME=${TARGET_DB_NAME:-"dbmicroservice"}  # Nombre cambiado a dbmicroservice
TARGET_DB_USER=${TARGET_DB_USER:-"$SOURCE_DB_USER"}
TARGET_DB_PASSWORD=${TARGET_DB_PASSWORD:-"$SOURCE_DB_PASSWORD"}

# Esquema de destino
TARGET_DB_SCHEMA=${TARGET_DB_SCHEMA:-"notification_service"}

# Archivos de script
MIGRATION_SCRIPT="migration.sql"
VERIFICATION_SCRIPT="verification.sql"
BACKUP_FILE="pre_migration_backup_$(date +%Y%m%d_%H%M%S).sql"

# Log de actividad
LOG_FILE="migration_$(date +%Y%m%d_%H%M%S).log"

# Función para escribir en log
log() {
    echo "[$(date +"%Y-%m-%d %H:%M:%S")] $1" | tee -a "$LOG_FILE"
}

# Función para ejecutar comandos SQL
execute_sql() {
    local db_host=$1
    local db_port=$2
    local db_name=$3
    local db_user=$4
    local db_pass=$5
    local sql=$6
    local output_file=$7

    PGPASSWORD="$db_pass" psql -h "$db_host" -p "$db_port" -d "$db_name" -U "$db_user" -c "$sql" ${output_file:+"> $output_file"}
}

# Función para ejecutar scripts SQL desde archivo
execute_sql_file() {
    local db_host=$1
    local db_port=$2
    local db_name=$3
    local db_user=$4
    local db_pass=$5
    local sql_file=$6
    local output_file=$7

    PGPASSWORD="$db_pass" psql -h "$db_host" -p "$db_port" -d "$db_name" -U "$db_user" -f "$sql_file" ${output_file:+"> $output_file"}
}

# Verificar si los scripts existen
if [ ! -f "$MIGRATION_SCRIPT" ]; then
    log "ERROR: Script de migración no encontrado: $MIGRATION_SCRIPT"
    log "Creando archivo de plantilla para migration.sql..."
    cat > "$MIGRATION_SCRIPT" << EOL
-- Script de migración de datos para el servicio de notificaciones
-- Este script migra datos del esquema principal del monolito al nuevo esquema de notificaciones

-- Crear esquema si no existe
CREATE SCHEMA IF NOT EXISTS ${TARGET_DB_SCHEMA};

-- Tablas requeridas en notification_service
-- Coloca aquí el contenido del script de migración
EOL
    log "Por favor, edita el archivo $MIGRATION_SCRIPT con el contenido adecuado y ejecuta nuevamente este script."
    exit 1
fi

if [ ! -f "$VERIFICATION_SCRIPT" ]; then
    log "ERROR: Script de verificación no encontrado: $VERIFICATION_SCRIPT"
    log "Creando archivo de plantilla para verification.sql..."
    cat > "$VERIFICATION_SCRIPT" << EOL
-- Script para verificar la migración de datos al esquema notification_service
-- Coloca aquí el contenido del script de verificación
EOL
    log "Por favor, edita el archivo $VERIFICATION_SCRIPT con el contenido adecuado y ejecuta nuevamente este script."
    exit 1
fi

# Mostrar información de configuración
log "Iniciando migración con la siguiente configuración:"
log "Origen: $SOURCE_DB_USER@$SOURCE_DB_HOST:$SOURCE_DB_PORT/$SOURCE_DB_NAME"
log "Destino: $TARGET_DB_USER@$TARGET_DB_HOST:$TARGET_DB_PORT/$TARGET_DB_NAME (esquema: $TARGET_DB_SCHEMA)"

# Verificar conexión a las bases de datos
log "Verificando conexión a la base de datos de origen..."
if ! execute_sql "$SOURCE_DB_HOST" "$SOURCE_DB_PORT" "$SOURCE_DB_NAME" "$SOURCE_DB_USER" "$SOURCE_DB_PASSWORD" "SELECT 1;" > /dev/null 2>&1; then
    log "ERROR: No se puede conectar a la base de datos de origen"
    exit 1
fi

log "Verificando conexión a la base de datos de destino..."
# Intentar conectarse a la base de datos de destino
if ! execute_sql "$TARGET_DB_HOST" "$TARGET_DB_PORT" "$TARGET_DB_NAME" "$TARGET_DB_USER" "$TARGET_DB_PASSWORD" "SELECT 1;" > /dev/null 2>&1; then
    log "ADVERTENCIA: La base de datos de destino no existe o no se puede conectar"
    log "Intentando crear la base de datos destino: $TARGET_DB_NAME"
    
    # Conectarse a postgres para crear la base de datos
    if ! PGPASSWORD="$TARGET_DB_PASSWORD" psql -h "$TARGET_DB_HOST" -p "$TARGET_DB_PORT" -U "$TARGET_DB_USER" -d postgres -c "CREATE DATABASE $TARGET_DB_NAME;" > /dev/null 2>&1; then
        log "ERROR: No se puede crear la base de datos de destino"
        exit 1
    fi
    
    log "Base de datos de destino creada exitosamente"
    
    # Verificar nuevamente la conexión
    if ! execute_sql "$TARGET_DB_HOST" "$TARGET_DB_PORT" "$TARGET_DB_NAME" "$TARGET_DB_USER" "$TARGET_DB_PASSWORD" "SELECT 1;" > /dev/null 2>&1; then
        log "ERROR: No se puede conectar a la base de datos de destino recién creada"
        exit 1
    fi
fi

# Preguntar confirmación
read -p "¿Desea realizar un backup antes de la migración? (s/n): " do_backup
if [[ "$do_backup" =~ ^[Ss]$ ]]; then
    log "Creando backup en $BACKUP_FILE..."
    
    # Decidir qué base de datos hacer backup
    if [ "$SOURCE_DB_NAME" = "$TARGET_DB_NAME" ]; then
        # Si es la misma base de datos, hacemos backup de toda la base
        PGPASSWORD="$SOURCE_DB_PASSWORD" pg_dump -h "$SOURCE_DB_HOST" -p "$SOURCE_DB_PORT" -U "$SOURCE_DB_USER" -d "$SOURCE_DB_NAME" > "$BACKUP_FILE"
    else
        # Si son bases diferentes, hacemos backup de la de destino si existe
        if execute_sql "$TARGET_DB_HOST" "$TARGET_DB_PORT" "$TARGET_DB_NAME" "$TARGET_DB_USER" "$TARGET_DB_PASSWORD" "SELECT 1;" > /dev/null 2>&1; then
            PGPASSWORD="$TARGET_DB_PASSWORD" pg_dump -h "$TARGET_DB_HOST" -p "$TARGET_DB_PORT" -U "$TARGET_DB_USER" -d "$TARGET_DB_NAME" > "$BACKUP_FILE"
        else
            log "No se puede hacer backup de la base de destino porque no existe todavía"
        fi
    fi
    
    log "Backup completado en $BACKUP_FILE"
fi

# Solicitar confirmación para continuar
read -p "¿Continuar con la migración? (s/n): " confirm_migration
if [[ ! "$confirm_migration" =~ ^[Ss]$ ]]; then
    log "Migración cancelada por el usuario"
    exit 0
fi

# Verificar si el esquema ya existe en la base de destino
log "Verificando si el esquema $TARGET_DB_SCHEMA ya existe en la base de destino..."
SCHEMA_EXISTS=$(execute_sql "$TARGET_DB_HOST" "$TARGET_DB_PORT" "$TARGET_DB_NAME" "$TARGET_DB_USER" "$TARGET_DB_PASSWORD" "SELECT EXISTS(SELECT 1 FROM information_schema.schemata WHERE schema_name = '$TARGET_DB_SCHEMA');" | grep -oP 't|f')

if [ "$SCHEMA_EXISTS" = "t" ]; then
    log "ADVERTENCIA: El esquema $TARGET_DB_SCHEMA ya existe en la base de destino"
    read -p "¿Desea continuar de todos modos? Esto podría sobrescribir datos existentes (s/n): " continue_with_existing
    if [[ ! "$continue_with_existing" =~ ^[Ss]$ ]]; then
        log "Migración cancelada por el usuario"
        exit 0
    fi
fi

# Ejecutar migración
log "Iniciando migración de datos..."
execute_sql_file "$TARGET_DB_HOST" "$TARGET_DB_PORT" "$TARGET_DB_NAME" "$TARGET_DB_USER" "$TARGET_DB_PASSWORD" "$MIGRATION_SCRIPT" "migration_output.log"
log "Migración completada. Detalles en migration_output.log"

# Verificar migración
log "Verificando migración..."
execute_sql_file "$TARGET_DB_HOST" "$TARGET_DB_PORT" "$TARGET_DB_NAME" "$TARGET_DB_USER" "$TARGET_DB_PASSWORD" "$VERIFICATION_SCRIPT" "verification_output.log"
log "Verificación completada. Detalles en verification_output.log"

# Informar resultados
if grep -q "ERROR" verification_output.log; then
    log "ADVERTENCIA: Se encontraron posibles problemas durante la verificación."
    log "Revise verification_output.log para más detalles."
else
    log "ÉXITO: La migración y verificación se completaron sin errores."
fi

# Mostrar estadísticas de migración
log "Estadísticas de migración:"
execute_sql "$TARGET_DB_HOST" "$TARGET_DB_PORT" "$TARGET_DB_NAME" "$TARGET_DB_USER" "$TARGET_DB_PASSWORD" \
    "SELECT 'Dispositivos' as tabla, COUNT(*) as registros FROM ${TARGET_DB_SCHEMA}.devices UNION ALL
     SELECT 'Tokens', COUNT(*) FROM ${TARGET_DB_SCHEMA}.notification_tokens UNION ALL
     SELECT 'Notificaciones', COUNT(*) FROM ${TARGET_DB_SCHEMA}.notifications UNION ALL
     SELECT 'Entregas', COUNT(*) FROM ${TARGET_DB_SCHEMA}.delivery_tracking;"

log "Migración finalizada."