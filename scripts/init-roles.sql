-- Run once on a new database as its bootstrap administrator.
\getenv database_name POSTGRES_DB
\getenv api_password JUDGIFY_API_PASSWORD
\getenv worker_password JUDGIFY_WORKER_PASSWORD
\getenv migrator_password JUDGIFY_MIGRATOR_PASSWORD

CREATE ROLE judgify_migrator WITH
  LOGIN PASSWORD :'migrator_password'
  CONNECTION LIMIT 5
  NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
CREATE ROLE judgify_api WITH
  LOGIN PASSWORD :'api_password'
  CONNECTION LIMIT 40
  NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
CREATE ROLE judgify_worker WITH
  LOGIN PASSWORD :'worker_password'
  CONNECTION LIMIT 40
  NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;

ALTER DATABASE :"database_name" OWNER TO judgify_migrator;
REVOKE ALL ON DATABASE :"database_name" FROM PUBLIC;
GRANT CONNECT ON DATABASE :"database_name"
  TO judgify_migrator, judgify_api, judgify_worker;

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
