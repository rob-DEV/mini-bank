DROP TABLE IF EXISTS accounts;
DROP TABLE IF EXISTS customers;

CREATE TABLE customers (
                           id             text        PRIMARY KEY,
                           first_name     text        NOT NULL,
                           last_name      text        NOT NULL,
                           email          text        NOT NULL UNIQUE,
                           date_of_birth  date        NOT NULL,
                           created_at     timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE accounts (
                          id              text        PRIMARY KEY,
                          customer_id     text        NOT NULL REFERENCES customers (id),
                          account_type    text        NOT NULL CHECK (account_type IN ('current', 'savings')),
                          sort_code       char(6)     NOT NULL,
                          account_number  char(8)     NOT NULL,
                          currency        char(3)     NOT NULL DEFAULT 'GBP',
                          balance_minor   bigint      NOT NULL DEFAULT 0,
                          status          text        NOT NULL DEFAULT 'open'
                              CHECK (status IN ('open', 'frozen', 'closed')),
                          created_at      timestamptz NOT NULL DEFAULT now(),

                          CONSTRAINT balance_non_negative CHECK (balance_minor >= 0),
                          CONSTRAINT uk_account_identifier UNIQUE (sort_code, account_number)
);

CREATE INDEX accounts_customer_id_idx ON accounts (customer_id);