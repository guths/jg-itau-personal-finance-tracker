#!/usr/bin/env bash
# Restores the "finance" database from a restic snapshot (the latest by default).
# Usage: scripts/restore.sh [snapshot-id]
#   RESTORE_DB=outro_banco  restores into another database (e.g. to inspect without touching data)
#   YES=1                   skips the confirmation
# Before overwriting, the current database is dumped to data/backups/ (a local safety copy).
source "$(dirname "$0")/lib.sh"
load_backup_env
check_backup_env

SNAPSHOT="${1:-latest}"
DB="${RESTORE_DB:-finance}"
SOURCE_DB="${BACKUP_DB:-finance}"

mysql_exec mysqladmin ping >/dev/null
restic snapshots --tag finance --host jg-finance-tracker --latest 5

if [ "${YES:-}" != "1" ]; then
  printf '\nIsso vai APAGAR o banco "%s" e recriá-lo a partir do snapshot "%s". Pare a API antes (Ctrl+C no make api).\nDigite o nome do banco para confirmar: ' "$DB" "$SNAPSHOT"
  read -r answer
  [ "$answer" = "$DB" ] || { echo "cancelado"; exit 1; }
fi

if mysql_exec mysql -N -e "SHOW DATABASES LIKE '$DB'" | grep -qx "$DB"; then
  mkdir -p data/backups
  safety="data/backups/pre-restore-$DB-$(date +%Y%m%d-%H%M%S).sql"
  log "cópia de segurança do banco atual em $safety"
  mysql_exec mysqldump --single-transaction --no-tablespaces --set-gtid-purged=OFF "$DB" > "$safety"
fi

log "restaurando '$SNAPSHOT' em '$DB'"
mysql_exec mysql -e "DROP DATABASE IF EXISTS \`$DB\`; CREATE DATABASE \`$DB\` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci; GRANT ALL PRIVILEGES ON \`$DB\`.* TO 'finance'@'%';"
restic dump --tag finance --host jg-finance-tracker "$SNAPSHOT" "$SOURCE_DB.sql" \
  | mysql_exec mysql --default-character-set=utf8mb4 "$DB"

log "✓ restaurado: $(mysql_exec mysql -N -e "SELECT COUNT(*) FROM \`$DB\`.transactions") lançamentos em '$DB'"
