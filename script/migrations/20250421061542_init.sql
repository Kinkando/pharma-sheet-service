-- migrate:up
CREATE TYPE pharma_sheet_approval_status AS ENUM (
  'APPROVED',
  'PENDING'
);

CREATE TYPE pharma_sheet_role AS ENUM (
  'ADMIN',
  'EDITOR',
  'VIEWER'
);

CREATE TABLE IF NOT EXISTS pharma_sheet_users (
  user_id UUID PRIMARY KEY,
  firebase_uid TEXT UNIQUE,
  email TEXT UNIQUE NOT NULL,
  display_name TEXT,
  image_url TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS pharma_sheet_warehouses (
  warehouse_id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS pharma_sheet_warehouse_users (
  warehouse_id TEXT NOT NULL,
  user_id UUID NOT NULL,
  role pharma_sheet_role NOT NULL,
  status pharma_sheet_approval_status NOT NULL DEFAULT 'PENDING',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT fk_warehouse_user_warehouse_id FOREIGN KEY (warehouse_id) REFERENCES pharma_sheet_warehouses (warehouse_id),
  CONSTRAINT fk_warehouse_user_user_id FOREIGN KEY (user_id) REFERENCES pharma_sheet_users (user_id),
  CONSTRAINT unique_warehouse_user UNIQUE (warehouse_id, user_id)
);

CREATE TABLE IF NOT EXISTS pharma_sheet_medicines (
  medication_id TEXT PRIMARY KEY,
  medical_name TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS pharma_sheet_medicine_brands (
  id UUID PRIMARY KEY,
  medication_id TEXT NOT NULL,
  trade_id TEXT NOT NULL,
  trade_name TEXT,
  blister_image_url TEXT,
  tablet_image_url TEXT,
  box_image_url TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT fk_brand_medication_id FOREIGN KEY (medication_id) REFERENCES pharma_sheet_medicines (medication_id) ON DELETE CASCADE,
  CONSTRAINT unique_brand UNIQUE (medication_id, trade_id)
);

CREATE TABLE IF NOT EXISTS pharma_sheet_medicine_blister_date_histories (
  id UUID PRIMARY KEY,
  warehouse_id TEXT NOT NULL,
  medication_id TEXT NOT NULL,
  brand_id UUID,
  blister_change_date DATE NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT fk_medicine_blister_date_history_warehouse_id FOREIGN KEY (warehouse_id) REFERENCES pharma_sheet_warehouses (warehouse_id) ON DELETE CASCADE,
  CONSTRAINT fk_medicine_blister_date_history_medication_id FOREIGN KEY (medication_id) REFERENCES pharma_sheet_medicines (medication_id) ON DELETE CASCADE,
  CONSTRAINT fk_medicine_blister_date_history_brand_id FOREIGN KEY (brand_id) REFERENCES pharma_sheet_medicine_brands (id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS pharma_sheet_medicine_houses (
  id UUID PRIMARY KEY,
  warehouse_id TEXT NOT NULL,
  medication_id TEXT NOT NULL,
  locker TEXT NOT NULL,
  floor INT NOT NULL,
  no INT NOT NULL,
  label TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT fk_house_warehouse_id FOREIGN KEY (warehouse_id) REFERENCES pharma_sheet_warehouses (warehouse_id) ON DELETE CASCADE,
  CONSTRAINT fk_house_medication_id FOREIGN KEY (medication_id) REFERENCES pharma_sheet_medicines (medication_id) ON DELETE CASCADE,
  CONSTRAINT unique_house UNIQUE (warehouse_id, medication_id, locker, floor, no)
);

CREATE TABLE IF NOT EXISTS pharma_sheet_warehouse_sheets (
  warehouse_id TEXT PRIMARY KEY,
  spreadsheet_id TEXT NOT NULL,
  medicine_sheet_id INT NOT NULL,
  medicine_brand_sheet_id INT NOT NULL,
  medicine_house_sheet_id INT NOT NULL,
  medicine_blister_date_history_sheet_id INT NOT NULL,
  latest_synced_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT fk_warehouse_sheet_warehouse_id FOREIGN KEY (warehouse_id) REFERENCES pharma_sheet_warehouses (warehouse_id) ON DELETE CASCADE,
  CONSTRAINT unique_pharma_sheet_warehouse_sheet UNIQUE (spreadsheet_id, medicine_sheet_id, medicine_brand_sheet_id, medicine_house_sheet_id, medicine_blister_date_history_sheet_id)
)

-- migrate:down
DROP TABLE IF EXISTS pharma_sheet_warehouse_sheets;
DROP TABLE IF EXISTS pharma_sheet_medicine_houses;
DROP TABLE IF EXISTS pharma_sheet_medicine_blister_date_histories;
DROP TABLE IF EXISTS pharma_sheet_medicine_brands;
DROP TABLE IF EXISTS pharma_sheet_medicines;
DROP TABLE IF EXISTS pharma_sheet_warehouse_users;
DROP TABLE IF EXISTS pharma_sheet_warehouses;
DROP TABLE IF EXISTS pharma_sheet_users;
DROP TYPE IF EXISTS pharma_sheet_role;
DROP TYPE IF EXISTS pharma_sheet_approval_status;
