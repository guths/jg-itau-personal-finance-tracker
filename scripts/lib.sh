# Shared helpers for the backup scripts. Sourced, not executed.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

# load_backup_env exports the restic settings from .env. The file is parsed line by line (not
# sourced) because other values, like MYSQL_DSN, are not valid shell syntax. Variables already
# set in the environment win.
load_backup_env() {
  [ -f .env ] || return 0
  local key value
  while IFS='=' read -r key value || [ -n "$key" ]; do
    case "$key" in
      RESTIC_*|B2_*|AWS_*) ;;
      *) continue ;;
    esac
    value="${value%\"}"; value="${value#\"}"
    value="${value%\'}"; value="${value#\'}"
    if [ -z "${!key:-}" ] && [ -n "$value" ]; then
      export "$key=$value"
    fi
  done < .env
}

require() {
  command -v "$1" >/dev/null 2>&1 || { echo "✗ '$1' não encontrado. $2" >&2; exit 1; }
}

check_backup_env() {
  require restic "Instale com: brew install restic"
  require docker "Instale/abra o Docker Desktop."
  : "${RESTIC_REPOSITORY:?defina RESTIC_REPOSITORY no .env (veja .env.example)}"
  if [ -z "${RESTIC_PASSWORD:-}" ] && [ -z "${RESTIC_PASSWORD_FILE:-}" ]; then
    echo "✗ defina RESTIC_PASSWORD no .env (veja .env.example)" >&2
    exit 1
  fi
}

# mysql_exec runs a client tool inside the MySQL container as root (local-only credentials from
# docker-compose.yml), passing the password through the environment to keep it off the command line.
mysql_exec() {
  docker compose exec -T -e MYSQL_PWD=root mysql "$@" -uroot
}

log() { printf '%s %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"; }
