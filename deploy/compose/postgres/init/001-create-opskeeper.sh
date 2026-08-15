#!/bin/sh
set -eu

: "${OPSK_DB_NAME:?OPSK_DB_NAME is required}"
: "${OPSK_DB_USER:?OPSK_DB_USER is required}"
: "${OPSK_DB_PASSWORD:?OPSK_DB_PASSWORD is required}"

psql \
  --set=ON_ERROR_STOP=1 \
  --username "$POSTGRES_USER" \
  --dbname "$POSTGRES_DB" \
  --set=opsk_db_name="$OPSK_DB_NAME" \
  --set=opsk_db_user="$OPSK_DB_USER" \
  --set=opsk_db_password="$OPSK_DB_PASSWORD" <<'SQL'
SELECT format(
  'CREATE ROLE %I LOGIN PASSWORD %L NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS',
  :'opsk_db_user',
  :'opsk_db_password'
)
WHERE NOT EXISTS (
  SELECT 1 FROM pg_roles WHERE rolname = :'opsk_db_user'
) \gexec

SELECT format(
  'CREATE DATABASE %I OWNER %I',
  :'opsk_db_name',
  :'opsk_db_user'
)
WHERE NOT EXISTS (
  SELECT 1 FROM pg_database WHERE datname = :'opsk_db_name'
) \gexec

REVOKE ALL ON DATABASE :"opsk_db_name" FROM PUBLIC;
GRANT CONNECT, TEMPORARY ON DATABASE :"opsk_db_name" TO :"opsk_db_user";
SQL
