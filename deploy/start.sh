#!/bin/sh
# Starts Kinlight in its container. With a backup bucket configured, it
# first restores the database from the backup if the volume has none (a
# new machine, or a lost volume), then runs the server under Litestream,
# which replicates every write and stops when the server does.
set -eu

: "${LITESTREAM_PATH:=kinlight}"
: "${LITESTREAM_REGION:=us-east-1}"
: "${LITESTREAM_ENDPOINT:=}"
: "${LITESTREAM_FORCE_PATH_STYLE:=false}"
export LITESTREAM_PATH LITESTREAM_REGION LITESTREAM_ENDPOINT LITESTREAM_FORCE_PATH_STYLE

if [ -z "${LITESTREAM_BUCKET:-}" ]; then
  echo "start: LITESTREAM_BUCKET isn't set, so the database is NOT being backed up" >&2
  exec /app/kinlight
fi

if [ ! -f "$DB_PATH" ]; then
  echo "start: no database at $DB_PATH; restoring from the backup if there is one" >&2
  litestream restore -config /etc/litestream.yml -if-replica-exists "$DB_PATH"
fi

exec litestream replicate -config /etc/litestream.yml -exec /app/kinlight
