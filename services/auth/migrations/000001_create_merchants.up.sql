CREATE TABLE merchants (
  id          VARCHAR(26) PRIMARY KEY,
  email       VARCHAR(255) UNIQUE NOT NULL,
  name        VARCHAR(255) NOT NULL,
  created_at  TIMESTAMPTZ DEFAULT NOW(),
  updated_at  TIMESTAMPTZ DEFAULT NOW()
);