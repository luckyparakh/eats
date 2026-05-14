BEGIN;

CREATE SCHEMA IF NOT EXISTS orders;

CREATE TABLE IF NOT EXISTS orders.customers (
    customer_uuid uuid PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL,
    address JSON NOT NULL,
    phone_number VARCHAR(50) NOT NULL
);

COMMIT;