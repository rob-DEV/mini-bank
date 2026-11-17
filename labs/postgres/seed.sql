INSERT INTO customers (id, first_name, last_name, email, date_of_birth) VALUES
                                                                            ('cus_001', 'Aoife',   'Murphy',    'aoife.murphy@example.com',  '1994-03-12'),
                                                                            ('cus_002', 'Ciaran',  'Kelly',     'ciaran.kelly@example.com',  '1988-11-02'),
                                                                            ('cus_003', 'Priya',   'Shah',      'priya.shah@example.com',    '1991-07-23'),
                                                                            ('cus_004', 'Tom',     'Henderson', 'tom.henderson@example.com', '1979-01-30'),
                                                                            ('cus_005', 'Niamh',   'O''Neill',  'niamh.oneill@example.com',  '2000-05-17'),
                                                                            ('cus_006', 'James',   'Okafor',    'james.okafor@example.com',  '1985-09-08'),
                                                                            ('cus_007', 'Sophie',  'Clarke',    'sophie.clarke@example.com', '1997-12-01'),
                                                                            ('cus_008', 'Mateusz', 'Nowak',     'mateusz.nowak@example.com', '1990-04-14'),
                                                                            ('cus_009', 'Hannah',  'Brown',     'hannah.brown@example.com',  '1983-06-26'),
                                                                            ('cus_010', 'Dev',     'Patel',     'dev.patel@example.com',     '1995-10-19');

INSERT INTO accounts (id, customer_id, account_type, sort_code, account_number, balance_minor)
SELECT 'acc_' || lpad(i::text, 3, '0'), 'cus_' || lpad(i::text, 3, '0'),
       'current', '990001', '1000' || lpad(i::text, 4, '0'), 100000 + i * 2500
FROM generate_series(1, 10) AS i;

INSERT INTO accounts (id, customer_id, account_type, sort_code, account_number, balance_minor)
SELECT 'acc_' || lpad((10 + i)::text, 3, '0'), 'cus_' || lpad(i::text, 3, '0'),
       'savings', '990001', '2000' || lpad(i::text, 4, '0'), 500000
FROM generate_series(1, 3) AS i;