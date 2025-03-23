#!/bin/bash

# Script para automatizar la migración de datos para el servicio de notificaciones
# Adaptado para PostgreSQL en Docker (Bitnami image)

set -e  # Salir inmediatamente si hay un error

# Variables de configuración para la base de datos PostgreSQL en Docker
DB_HOST="localhost"
DB_PORT="54322"  # Puerto mapeado del contenedor Bitnami
DB_USER="postgres"
DB_PASSWORD="postgres"
DB_NAME="dbmicroservice"
DB_SCHEMA="notification_service"

# Archivos de script
MIGRATION_SCRIPT="migration_postgres.sql"
VERIFICATION_SCRIPT="verification_postgres.sql"
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
log "Host: $DB_HOST:$DB_PORT"
log "Usuario: $DB_USER"
log "Base de datos destino: $DB_NAME (esquema: $DB_SCHEMA)"

# Verificar conexión a PostgreSQL
log "Verificando conexión a PostgreSQL..."
if ! PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "postgres" -c "SELECT 1;" > /dev/null 2>&1; then
    log "ERROR: No se puede conectar a PostgreSQL. Verifica que el contenedor esté en ejecución y los datos de conexión sean correctos."
    exit 1
fi

log "Conexión a PostgreSQL establecida."

# Verificar si la base de datos destino existe y crearla si no
log "Verificando existencia de la base de datos $DB_NAME..."
DB_EXISTS=$(PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "postgres" -t -c "SELECT 1 FROM pg_database WHERE datname='$DB_NAME';" | grep -c "1")

if [ "$DB_EXISTS" -eq "0" ]; then
    log "La base de datos $DB_NAME no existe. Creándola..."
    PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "postgres" -c "CREATE DATABASE $DB_NAME;"
    log "Base de datos $DB_NAME creada."
else
    log "La base de datos $DB_NAME ya existe."
fi

# Preguntar confirmación
read -p "¿Desea realizar un backup antes de la migración? (s/n): " do_backup
if [[ "$do_backup" =~ ^[Ss]$ ]]; then
    log "Creando backup en $BACKUP_FILE..."
    
    # Si la base de datos ya existe, hacemos backup
    if [ "$DB_EXISTS" -eq "1" ]; then
        log "Intentando backup con opción --no-check-version para evitar error de versión..."
        # Intentar con --no-check-version
        PGPASSWORD="$DB_PASSWORD" pg_dump --no-check-version -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" > "$BACKUP_FILE" 2>/tmp/pg_dump_error.log || {
            BACKUP_ERROR=$?
            log "Error al crear backup: $(cat /tmp/pg_dump_error.log)"
            log "Continuando sin backup debido a incompatibilidad de versiones"
        }
        
        if [ -s "$BACKUP_FILE" ]; then
            log "Backup completado en $BACKUP_FILE"
        fi
    else
        log "No se puede hacer backup porque la base de datos todavía no existe."
    fi
fi

# Solicitar confirmación para continuar
read -p "¿Continuar con la migración? (s/n): " confirm_migration
if [[ ! "$confirm_migration" =~ ^[Ss]$ ]]; then
    log "Migración cancelada por el usuario"
    exit 0
fi

# Ejecutar migración
log "Iniciando migración de esquema y tablas..."
PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -f "$MIGRATION_SCRIPT" > "migration_output.log" 2>&1
log "Migración completada. Detalles en migration_output.log"

# Verificar migración
log "Verificando migración..."
PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -f "$VERIFICATION_SCRIPT" > "verification_output.log" 2>&1
log "Verificación completada. Detalles en verification_output.log"

# Mostrar estadísticas de migración
log "Estadísticas de migración:"
PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c \
    "SELECT 'Dispositivos' as tabla, COUNT(*) as registros FROM ${DB_SCHEMA}.devices UNION ALL
     SELECT 'Tokens', COUNT(*) FROM ${DB_SCHEMA}.notification_tokens UNION ALL
     SELECT 'Notificaciones', COUNT(*) FROM ${DB_SCHEMA}.notifications UNION ALL
     SELECT 'Entregas', COUNT(*) FROM ${DB_SCHEMA}.delivery_tracking;"

log "Migración finalizada."