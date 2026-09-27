#!/usr/bin/env bash
# Schedules scripts/backup.sh every day at 21:00 with launchd (macOS). If the Mac is asleep at
# that time, launchd runs it on wake. Log: data/backup.log.
# Usage: scripts/backup-schedule.sh [--remove]
source "$(dirname "$0")/lib.sh"

LABEL="com.jgfinance.backup"
PLIST="$HOME/Library/LaunchAgents/$LABEL.plist"

launchctl bootout "gui/$(id -u)/$LABEL" 2>/dev/null || true
if [ "${1:-}" = "--remove" ]; then
  rm -f "$PLIST"
  echo "✓ agendamento removido"
  exit 0
fi

mkdir -p "$HOME/Library/LaunchAgents" data
cat > "$PLIST" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>$LABEL</string>
  <key>ProgramArguments</key>
  <array><string>/bin/bash</string><string>$REPO_ROOT/scripts/backup.sh</string></array>
  <key>StartCalendarInterval</key>
  <dict><key>Hour</key><integer>21</integer><key>Minute</key><integer>0</integer></dict>
  <key>EnvironmentVariables</key>
  <dict><key>PATH</key><string>/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin</string></dict>
  <key>StandardOutPath</key><string>$REPO_ROOT/data/backup.log</string>
  <key>StandardErrorPath</key><string>$REPO_ROOT/data/backup.log</string>
</dict>
</plist>
EOF
launchctl bootstrap "gui/$(id -u)" "$PLIST"
echo "✓ backup agendado todo dia às 21:00 (log em data/backup.log)"
