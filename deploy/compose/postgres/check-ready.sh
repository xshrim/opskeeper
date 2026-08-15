#!/bin/sh
set -eu

pg_isready --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" >/dev/null

admin_count="$(
  psql \
    --quiet \
    --tuples-only \
    --no-align \
    --username "$POSTGRES_USER" \
    --dbname "$POSTGRES_DB" \
    --set=admin_user="$POSTGRES_USER" <<'SQL'
SELECT count(*)
FROM pg_roles
WHERE rolname = :'admin_user'
  AND rolsuper;
SQL
)"

business_role_count="$(
  psql \
    --quiet \
    --tuples-only \
    --no-align \
    --username "$POSTGRES_USER" \
    --dbname "$POSTGRES_DB" \
    --set=opsk_db_user="$OPSK_DB_USER" <<'SQL'
SELECT count(*)
FROM pg_roles
WHERE rolname = :'opsk_db_user'
  AND NOT rolsuper
  AND NOT rolcreatedb
  AND NOT rolcreaterole
  AND NOT rolreplication
  AND NOT rolbypassrls;
SQL
)"

business_database_count="$(
  psql \
    --quiet \
    --tuples-only \
    --no-align \
    --username "$POSTGRES_USER" \
    --dbname "$POSTGRES_DB" \
    --set=opsk_db_name="$OPSK_DB_NAME" \
    --set=opsk_db_user="$OPSK_DB_USER" <<'SQL'
SELECT count(*)
FROM pg_database AS database
JOIN pg_roles AS owner ON owner.oid = database.datdba
WHERE database.datname = :'opsk_db_name'
  AND owner.rolname = :'opsk_db_user';
SQL
)"

test "$admin_count" = "1"
test "$business_role_count" = "1"
test "$business_database_count" = "1"
