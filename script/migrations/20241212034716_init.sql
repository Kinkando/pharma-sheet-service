-- migrate:up
CREATE TYPE role AS ENUM (
  'ADMIN',
  'EDITOR',
  'VIEWER'
);

CREATE TABLE users (
  user_id UUID PRIMARY KEY,
  firebase_uid TEXT UNIQUE,
  email TEXT UNIQUE NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ
);

CREATE TABLE warehouses (
  warehouse_id UUID PRIMARY KEY,
  name TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ
);

CREATE TABLE warehouse_users (
  warehouse_id UUID NOT NULL,
  user_id UUID NOT NULL,
  role role NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ,
  CONSTRAINT fk_warehouse_user_warehouse_id FOREIGN KEY (warehouse_id) REFERENCES warehouses (warehouse_id),
  CONSTRAINT fk_warehouse_user_user_id FOREIGN KEY (user_id) REFERENCES users (user_id),
  CONSTRAINT unique_warehouse_user UNIQUE (warehouse_id, user_id)
);

CREATE TABLE lockers (
  locker_id UUID PRIMARY KEY,
  warehouse_id UUID NOT NULL,
  name TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ,
  CONSTRAINT fk_locker_warehouse_id FOREIGN KEY (warehouse_id) REFERENCES warehouses (warehouse_id)
);

CREATE TABLE medicines (
  medicine_id UUID PRIMARY KEY,
  warehouse_id UUID NOT NULL,
  locker_id UUID NOT NULL,
  floor INT NOT NULL,
  no INT NOT NULL,
  address TEXT NOT NULL,
  description TEXT NOT NULL,
  medical_name TEXT NOT NULL,
  label TEXT NOT NULL,
  image_url TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ,
  CONSTRAINT fk_medicine_warehouse_id FOREIGN KEY (warehouse_id) REFERENCES warehouses (warehouse_id),
  CONSTRAINT fk_medicine_locker_id FOREIGN KEY (locker_id) REFERENCES lockers (locker_id)
);

-- migrate:down
DROP TABLE IF EXISTS medicines;
DROP TABLE IF EXISTS lockers;
DROP TABLE IF EXISTS warehouse_users;
DROP TABLE IF EXISTS warehouses;
DROP TABLE IF EXISTS users;
DROP TYPE role;
