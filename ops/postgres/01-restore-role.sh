#!/bin/sh
set -eu
password=$(cat /run/secrets/mini_restore_password)
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" --set=password="$password" <<'SQL'
CREATE ROLE mini_restore LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS PASSWORD :'password';
ALTER DATABASE mini_inference_restore OWNER TO mini_restore;
ALTER SCHEMA public OWNER TO mini_restore;
SQL
unset password
