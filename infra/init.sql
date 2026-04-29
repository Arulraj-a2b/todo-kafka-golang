-- Provisions the two databases on first Postgres boot. Each service runs its
-- own migrations against its own database.
CREATE DATABASE auth_db;
CREATE DATABASE todo_db;
