CREATE TABLE transactions (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    checkout_id   VARCHAR(255) UNIQUE NOT NULL,
    amount        DECIMAL(10,2) NOT NULL,
    currency      VARCHAR(3)   NOT NULL DEFAULT 'PHP',
    status        VARCHAR(32)  NOT NULL DEFAULT 'PENDING',
    buyer_name    VARCHAR(255),
    redirect_url  TEXT,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_transactions_created_at ON transactions (created_at DESC);

CREATE OR REPLACE FUNCTION set_updated_at() RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER transactions_set_updated_at
    BEFORE UPDATE ON transactions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();