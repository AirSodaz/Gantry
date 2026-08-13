-- Gantry development database initialization
-- This file runs on first docker-compose up

-- Enable extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Schema for Gantry platform
CREATE SCHEMA IF NOT EXISTS gantry;

-- Set search path
ALTER DATABASE gantry SET search_path TO gantry, public;
