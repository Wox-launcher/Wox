#!/bin/bash

PID="$1"
OLD_PATH="$2"
NEW_PATH="$3"
LOG_FILE="$4"

if [ -z "$PID" ] || [ -z "$OLD_PATH" ] || [ -z "$NEW_PATH" ]; then
  echo "$(date "+%Y-%m-%d %H:%M:%S") missing required args" >> "$LOG_FILE"
  exit 1
fi

if [ -z "$LOG_FILE" ]; then
  LOG_FILE="/tmp/wox_update.log"
fi

log() {
  local now
  now=$(date "+%Y-%m-%d %H:%M:%S")
  echo "$now $1" >> "$LOG_FILE"
  echo "$1"
}

log "Update process started"
log "Old path: $OLD_PATH"
log "New path: $NEW_PATH"

while ps -p "$PID" > /dev/null 2>&1 || pgrep -f "$(basename "$OLD_PATH")" > /dev/null 2>&1; do
  sleep 1
done

log "Application exited, replacing executable"
cp "$NEW_PATH" "$OLD_PATH"
if [ $? -ne 0 ]; then
  log "Failed to copy executable, trying with sudo"
  sudo cp "$NEW_PATH" "$OLD_PATH"
  if [ $? -ne 0 ]; then
    log "ERROR: Failed to copy executable"
    exit 1
  fi
fi

chmod +x "$OLD_PATH" || sudo chmod +x "$OLD_PATH"

log "Launching updated application"
"$OLD_PATH" >/dev/null 2>&1 &
log "Update process completed"
