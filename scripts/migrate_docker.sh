#!/bin/bash

# Script para automatizar la migración de datos para el servicio de notificaciones en Docker
# Ejecutar desde un entorno seguro con acceso a las bases de datos

set -e  # Salir inmediatamente si hay un error

# Variables de configuración ajustadas para Docker
SOURCE_DB_HOST="localhost"
SOURCE_DB_PORT="54322"  # Puerto mapeado del contenedor
SOURCE_DB_USER="postgres"
SOURCE_DB_PASSWORD="postgres"
TARGET_DB_NAME="dbmicroservice"
TARGET_DB_SCHEMA="notification_service"

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

# Mostrar información de configuración
log "Iniciando migración con la siguiente configuración:"
log "Host: $SOURCE_DB_HOST:$SOURCE_DB_PORT"
log "Usuario: $SOURCE_DB_USER"
log "Base de datos destino: $TARGET_DB_NAME (esquema: $TARGET_DB_SCHEMA)"

# Verificar conexión a PostgreSQL
log "Verificando conexión a PostgreSQL..."
if ! execute_sql "$SOURCE_DB_HOST" "$SOURCE_DB_PORT" "postgres" "$SOURCE_DB_USER" "$SOURCE_DB_PASSWORD" "SELECT 1;" > /dev/null 2>&1; then
    log "ERROR: No se puede conectar a PostgreSQL. Verifica que el contenedor esté en ejecución y los datos de conexión sean correctos."
    exit 1
fi

log "Conexión a PostgreSQL establecida."

# Verificar si la base de datos destino existe y crearla si no
log "Verificando existencia de la base de datos $TARGET_DB_NAME..."
DB_EXISTS=$(execute_sql "$SOURCE_DB_HOST" "$SOURCE_DB_PORT" "postgres" "$SOURCE_DB_USER" "$SOURCE_DB_PASSWORD" "SELECT 1 FROM pg_database WHERE datname='$TARGET_DB_NAME';" | grep -c "1")

if [ "$DB_EXISTS" -eq "0" ]; then
    log "La base de datos $TARGET_DB_NAME no existe. Creándola..."
    execute_sql "$SOURCE_DB_HOST" "$SOURCE_DB_PORT" "postgres" "$SOURCE_DB_USER" "$SOURCE_DB_PASSWORD" "CREATE DATABASE $TARGET_DB_NAME;"
    log "Base de datos $TARGET_DB_NAME creada."
else
    log "La base de datos $TARGET_DB_NAME ya existe."
fi

# Preguntar confirmación
read -p "¿Desea realizar un backup antes de la migración? (s/n): " do_backup
if [[ "$do_backup" =~ ^[Ss]$ ]]; then
    log "Creando backup en $BACKUP_FILE..."
    PGPASSWORD="$SOURCE_DB_PASSWORD" pg_dump -h "$SOURCE_DB_HOST" -p "$SOURCE_DB_PORT" -U "$SOURCE_DB_USER" -d "$TARGET_DB_NAME" > "$BACKUP_FILE"
    log "Backup completado en $BACKUP_FILE"
fi

# Solicitar confirmación para continuar
read -p "¿Continuar con la migración? (s/n): " confirm_migration
if [[ ! "$confirm_migration" =~ ^[Ss]$ ]]; then
    log "Migración cancelada por el usuario"
    exit 0
fi

# Ejecutar migración
log "Iniciando migración de esquema y tablas..."
execute_sql_file "$SOURCE_DB_HOST" "$SOURCE_DB_PORT" "$TARGET_DB_NAME" "$SOURCE_DB_USER" "$SOURCE_DB_PASSWORD" "$MIGRATION_SCRIPT" "migration_output.log"
log "Migración completada. Detalles en migration_output.log"

# Verificar migración
log "Verificando migración..."
execute_sql_file "$SOURCE_DB_HOST" "$SOURCE_DB_PORT" "$TARGET_DB_NAME" "$SOURCE_DB_USER" "$SOURCE_DB_PASSWORD" "$VERIFICATION_SCRIPT" "verification_output.log"
log "Verificación completada. Detalles en verification_output.log"

# Mostrar estadísticas de migración
log "Estadísticas de migración:"
execute_sql "$SOURCE_DB_HOST" "$SOURCE_DB_PORT" "$TARGET_DB_NAME" "$SOURCE_DB_USER" "$SOURCE_DB_PASSWORD" \
    "SELECT 'Dispositivos' as tabla, COUNT(*) as registros FROM ${TARGET_DB_SCHEMA}.devices UNION ALL
     SELECT 'Tokens', COUNT(*) FROM ${TARGET_DB_SCHEMA}.notification_tokens UNION ALL
     SELECT 'Notificaciones', COUNT(*) FROM ${TARGET_DB_SCHEMA}.notifications UNION ALL
     SELECT 'Entregas', COUNT(*) FROM ${TARGET_DB_SCHEMA}.delivery_tracking;"

log "Migración finalizada."
