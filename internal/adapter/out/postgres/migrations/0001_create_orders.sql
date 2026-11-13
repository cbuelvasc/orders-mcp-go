CREATE TABLE orders
(
    id          VARCHAR(20)    PRIMARY KEY,
    customer_id VARCHAR(20)    NOT NULL,
    status      VARCHAR(20)    NOT NULL CHECK (status IN ('CREATED', 'PAID', 'SHIPPED', 'DELIVERED', 'CANCELLED')),
    total       NUMERIC(12, 2) NOT NULL CHECK (total >= 0),
    currency    CHAR(3)        NOT NULL,
    updated_at  TIMESTAMPTZ    NOT NULL
);

-- Supports listRecentOrders: WHERE customer_id = ? ORDER BY updated_at DESC LIMIT ?
CREATE INDEX idx_orders_customer_recent ON orders (customer_id, updated_at DESC);
