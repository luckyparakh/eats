BEGIN;
CREATE SCHEMA IF NOT EXISTS orders;
CREATE TABLE IF NOT EXISTS orders.customers (
    customer_uuid UUID NOT NULL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL,
    address json NOT NULL,
    phone_number VARCHAR(50) NOT NULL
);
COMMIT;