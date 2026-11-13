-- Demo data: ORD-1000 .. ORD-1059 for customers CUS-1 .. CUS-6
INSERT INTO orders (id, customer_id, status, total, currency, updated_at)
SELECT 'ORD-' || n,
       'CUS-' || (n % 6 + 1),
       (ARRAY ['CREATED', 'PAID', 'SHIPPED', 'DELIVERED', 'CANCELLED'])[n % 5 + 1],
       15 + (n % 17) * 7.5,
       'USD',
       now() - ((1060 - n) || ' hours')::interval
FROM generate_series(1000, 1059) AS n;
