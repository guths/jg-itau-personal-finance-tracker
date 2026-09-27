#!/usr/bin/env bash
# Backup of the "finance" database: mysqldump → restic (encrypted, deduplicated) → B2/R2.
# Usage: scripts/backup.sh [--init]   (--init creates the restic repository the first time)
source "$(dirname "$0")/lib.sh"
load_backup_env
check_backup_env

if [ "${1:-}" = "--init" ]; then
  log "criando o repositório restic em $RESTIC_REPOSITORY"
  restic init
  exit 0
fi

DB="${BACKUP_DB:-finance}"
log "backup do banco '$DB' → $RESTIC_REPOSITORY"
mysql_exec mysqladmin ping >/dev/null

# --single-transaction: consistent snapshot without locking the tables the app is using.
mysql_exec mysqldump --single-transaction --no-tablespaces --routines --triggers \
    --set-gtid-purged=OFF --default-character-set=utf8mb4 "$DB" \
  | restic backup --stdin --stdin-filename "$DB.sql" --tag finance --host jg-finance-tracker

log "aplicando a retenção (7 diários, 8 semanais, 12 mensais, 3 anuais)"
restic forget --tag finance --host jg-finance-tracker \
  --keep-daily 7 --keep-weekly 8 --keep-monthly 12 --keep-yearly 3 --prune

log "✓ backup concluído"
