\set ON_ERROR_STOP on
\getenv database_name POSTGRES_DB
\getenv api_password JUDGIFY_API_PASSWORD
\getenv worker_password JUDGIFY_WORKER_PASSWORD
\getenv migrator_password JUDGIFY_MIGRATOR_PASSWORD

SELECT 'CREATE ROLE judgify_migrator LOGIN'
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'judgify_migrator')
\gexec
SELECT 'CREATE ROLE judgify_api LOGIN'
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'judgify_api')
\gexec
SELECT 'CREATE ROLE judgify_worker LOGIN'
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'judgify_worker')
\gexec

SELECT format(
  'ALTER ROLE judgify_migrator WITH LOGIN PASSWORD %L CONNECTION LIMIT 5 NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS',
  :'migrator_password'
) \gexec
SELECT format(
  'ALTER ROLE judgify_api WITH LOGIN PASSWORD %L CONNECTION LIMIT 40 NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS',
  :'api_password'
) \gexec
SELECT format(
  'ALTER ROLE judgify_worker WITH LOGIN PASSWORD %L CONNECTION LIMIT 40 NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS',
  :'worker_password'
) \gexec

SELECT format('ALTER DATABASE %I OWNER TO judgify_migrator', :'database_name') \gexec
SELECT format('REVOKE ALL ON DATABASE %I FROM PUBLIC', :'database_name') \gexec
SELECT format('GRANT CONNECT ON DATABASE %I TO judgify_migrator, judgify_api, judgify_worker', :'database_name') \gexec

ALTER SCHEMA public OWNER TO judgify_migrator;
REVOKE ALL ON SCHEMA public FROM PUBLIC;
GRANT USAGE, CREATE ON SCHEMA public TO judgify_migrator;
GRANT USAGE ON SCHEMA public TO judgify_api, judgify_worker;

ALTER DEFAULT PRIVILEGES FOR ROLE judgify_migrator IN SCHEMA public
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO judgify_api, judgify_worker;
ALTER DEFAULT PRIVILEGES FOR ROLE judgify_migrator IN SCHEMA public
  GRANT USAGE, SELECT ON SEQUENCES TO judgify_api, judgify_worker;
ALTER DEFAULT PRIVILEGES FOR ROLE judgify_migrator IN SCHEMA public
  REVOKE EXECUTE ON FUNCTIONS FROM PUBLIC;

ALTER ROLE judgify_migrator SET search_path = public;
ALTER ROLE judgify_api SET search_path = public;
ALTER ROLE judgify_worker SET search_path = public;
ALTER ROLE judgify_migrator SET statement_timeout = '15min';
ALTER ROLE judgify_migrator SET lock_timeout = '30s';
ALTER ROLE judgify_migrator SET idle_in_transaction_session_timeout = '60s';
ALTER ROLE judgify_api SET statement_timeout = '30s';
ALTER ROLE judgify_api SET lock_timeout = '5s';
ALTER ROLE judgify_api SET idle_in_transaction_session_timeout = '15s';
ALTER ROLE judgify_worker SET statement_timeout = '60s';
ALTER ROLE judgify_worker SET lock_timeout = '5s';
ALTER ROLE judgify_worker SET idle_in_transaction_session_timeout = '15s';
