#!/bin/sh
set -eu

MIGRATION_NAME="20260428233000_init"
MIGRATION_FILE="prisma/migrations/${MIGRATION_NAME}/migration.sql"

set +e
MIGRATE_OUTPUT="$(npx prisma migrate deploy 2>&1)"
MIGRATE_STATUS=$?
set -e

printf '%s\n' "$MIGRATE_OUTPUT"

if [ "$MIGRATE_STATUS" -ne 0 ]; then
  case "$MIGRATE_OUTPUT" in
    *P3005*)
      echo "Existing shared schema detected; applying only webhook migration ${MIGRATION_NAME} and baselining Prisma history."
      npx prisma db execute --file "$MIGRATION_FILE"
      npx prisma migrate resolve --applied "$MIGRATION_NAME"
      ;;
    *)
      exit "$MIGRATE_STATUS"
      ;;
  esac
fi

if [ "$#" -eq 0 ]; then
  set -- node dist/main
fi

exec "$@"
