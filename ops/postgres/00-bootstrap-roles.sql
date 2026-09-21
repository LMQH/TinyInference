\set ON_ERROR_STOP on

-- Runs exactly once as the built-in postgres administrator. mini_migrator keeps
-- only the transient authority required to create and transfer migration objects;
-- the migration job installs runtime passwords and removes that authority.
DO $bootstrap$
DECLARE
  password text := btrim(pg_read_file('/run/secrets/mini_migrator_password'), E' \t\r\n');
BEGIN
  IF password !~ '^[A-Za-z0-9_.~-]+$' THEN
    RAISE EXCEPTION 'invalid mini_migrator password secret';
  END IF;
END
$bootstrap$;
SELECT btrim(pg_read_file('/run/secrets/mini_migrator_password'), E' \t\r\n') AS migrator_password \gset
CREATE ROLE mini_owner NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
CREATE ROLE mini_migrator LOGIN PASSWORD :'migrator_password' NOSUPERUSER NOCREATEDB CREATEROLE NOREPLICATION NOBYPASSRLS;
\unset migrator_password
CREATE ROLE mini_api LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
CREATE ROLE mini_retention LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
CREATE ROLE mini_backup LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
CREATE ROLE mini_restore LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
CREATE ROLE mini_controller_epoch LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
GRANT mini_owner TO mini_migrator WITH ADMIN OPTION;
ALTER SCHEMA public OWNER TO mini_owner;
DO $database_privileges$
BEGIN
  EXECUTE format('GRANT CREATE ON DATABASE %I TO mini_migrator, mini_owner', current_database());
  EXECUTE format('GRANT CONNECT ON DATABASE %I TO mini_api, mini_retention, mini_backup, mini_restore, mini_controller_epoch', current_database());
  EXECUTE format('GRANT CONNECT ON DATABASE %I TO mini_migrator WITH GRANT OPTION', current_database());
END
$database_privileges$;
