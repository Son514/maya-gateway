DROP TRIGGER IF EXISTS transactions_set_updated_at ON transactions;
DROP FUNCTION IF EXISTS set_updated_at();
DROP TABLE IF EXISTS transactions;