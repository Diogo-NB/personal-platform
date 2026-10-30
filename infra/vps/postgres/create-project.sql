-- Creates one project's login role and the database it owns.
-- Usage: see ../README.md. The password is prompted for and sent hashed.
\set ON_ERROR_STOP on

\if :{?project}
\else
  DO $$ BEGIN RAISE EXCEPTION 'missing -v project=<name>'; END $$;
\endif

CREATE ROLE :"project" LOGIN;
CREATE DATABASE :"project" OWNER :"project";
-- New databases grant CONNECT and TEMP to PUBLIC, which would let every
-- other project's role connect to this one.
REVOKE ALL ON DATABASE :"project" FROM PUBLIC;

\password :project
