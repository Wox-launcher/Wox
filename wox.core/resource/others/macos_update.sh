#!/bin/bash

LOG_FILE="$1"
VERSION="$2"
APP_PATH="$3"
PID="$4"
OLD_PATH="$5"

if [ -z "$LOG_FILE" ]; then
  LOG_FILE="/tmp/wox_update.log"
fi

if [ -z "$APP_PATH" ] || [ -z "$PID" ]; then
  echo "$(date "+%Y-%m-%d %H:%M:%S") missing required args" >> "$LOG_FILE"
  exit 1
fi

log() {
  local now
  now=$(date "+%Y-%m-%d %H:%M:%S")
  echo "$now $1" >> "$LOG_FILE"
  echo "$1"
}

log "Update process started for version $VERSION"
log "Extracted app path: $APP_PATH"

log "Waiting for application with PID $PID to exit..."
WAIT_COUNT=0

is_process_running() {
  ps -p "$PID" > /dev/null 2>&1
  return $?
}

while is_process_running; do
  if [ $((WAIT_COUNT % 5)) -eq 0 ]; then
    log "Still waiting for application to exit after ${WAIT_COUNT}s"
  fi

  if [ $WAIT_COUNT -eq 30 ]; then
    log "WARNING: Waited for 30 seconds. Forcing continue."
    break
  fi

  sleep 1
  WAIT_COUNT=$((WAIT_COUNT + 1))
done

log "Application has exited or timeout reached after ${WAIT_COUNT}s"

APP_NAME=$(basename "$APP_PATH")
TARGET_DIR=""
TARGET_APP_NAME="$APP_NAME"
TARGET_APP_PATH=""

if [ -n "$OLD_PATH" ]; then
  if [[ "$OLD_PATH" == *".app/"* ]]; then
    OLD_APP_PATH="${OLD_PATH%%.app/*}.app"
    TARGET_APP_NAME=$(basename "$OLD_APP_PATH")
    TARGET_DIR=$(dirname "$OLD_APP_PATH")
  elif [[ "$OLD_PATH" == *.app ]]; then
    TARGET_APP_NAME=$(basename "$OLD_PATH")
    TARGET_DIR=$(dirname "$OLD_PATH")
  else
    TARGET_DIR=$(dirname "$OLD_PATH")
  fi
fi

if [ -z "$TARGET_DIR" ]; then
  TARGET_DIR="/Applications"
fi

TARGET_APP_PATH="$TARGET_DIR/$TARGET_APP_NAME"
log "Target app path: $TARGET_APP_PATH"

log "Copying $APP_NAME to $TARGET_DIR/"

if [ -d "$TARGET_APP_PATH" ]; then
  log "Removing existing app: $TARGET_APP_PATH"
  rm -rf "$TARGET_APP_PATH"
  if [ $? -ne 0 ]; then
    log "Failed to remove existing app, trying with sudo"
    sudo rm -rf "$TARGET_APP_PATH"
    if [ $? -ne 0 ]; then
      log "ERROR: Failed to remove existing app even with sudo"
      exit 1
    fi
  fi
fi

log "Copying app to target folder"
cp -R "$APP_PATH" "$TARGET_DIR/"
if [ $? -ne 0 ]; then
  log "Failed to copy app, trying with sudo"
  sudo cp -R "$APP_PATH" "$TARGET_DIR/"
  if [ $? -ne 0 ]; then
    log "ERROR: Failed to copy app to target folder"
    exit 1
  fi
fi

if [ "$APP_NAME" != "$TARGET_APP_NAME" ] && [ -d "$TARGET_DIR/$APP_NAME" ]; then
  rm -rf "$TARGET_APP_PATH"
  mv "$TARGET_DIR/$APP_NAME" "$TARGET_APP_PATH"
fi

if [ ! -d "$TARGET_APP_PATH" ]; then
  log "ERROR: App was not copied to target folder"
  exit 1
fi

log "App copied successfully to target folder"

log "Cleaning up temporary directory"
rm -rf "$(dirname "$APP_PATH")"

log "Opening new application: $TARGET_APP_PATH"
open "$TARGET_APP_PATH" || open -a "$TARGET_APP_PATH"

log "Update process completed successfully"
