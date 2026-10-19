-- Provisions the MariaDB schemas Proviant needs. The application never creates
-- schemas: it fails fast at startup when a configured schema is missing.
--
-- Run against an existing server:
--   mysql -h <host> -u root -p < scripts/mariadb-init.sql
--
-- The Docker compose file mounts this at /docker-entrypoint-initdb.d/, so a
-- fresh volume is provisioned automatically. The MariaDB entrypoint only runs
-- init scripts on an empty data directory: on an existing volume run this file
-- manually.
--
-- Adjust the user and host in the GRANT statements to match
-- PROVIANT_DATABASE_MARIADB_USER and how the app connects ('%' covers the
-- container network).

-- Main application schema.
CREATE DATABASE IF NOT EXISTS proviant CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- Separate schema for the OpenFoodFacts bulk dump. Only required when
-- openfoodfacts.dumpImportEnabled is true.
CREATE DATABASE IF NOT EXISTS off_dump CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

GRANT ALL PRIVILEGES ON proviant.* TO 'proviant'@'%';
GRANT ALL PRIVILEGES ON off_dump.* TO 'proviant'@'%';
FLUSH PRIVILEGES;
