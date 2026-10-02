package db

import (
	"context"
	"database/sql"
)

// Transaction is a checkout record as stored locally. Amount and currency come
// from the create request, since Maya's create response returns neither.
type Transaction struct {
	CheckoutID  string
	Amount      float64
	Currency    string
	BuyerName   string
	RedirectURL string
}

// InsertCheckout records a newly created checkout. status is left to its column
// default of PENDING.
func InsertCheckout(ctx context.Context, pool *sql.DB, t Transaction) error {
	_, err := pool.ExecContext(ctx, `
		INSERT INTO transactions (checkout_id, amount, currency, buyer_name, redirect_url)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5)`,
		t.CheckoutID, t.Amount, t.Currency, t.BuyerName, t.RedirectURL,
	)
	return err
}

// UpdateStatus records Maya's paymentStatus for a checkout. It reports whether a
// row matched, so callers can tell an unknown checkout from a write failure.
func UpdateStatus(ctx context.Context, pool *sql.DB, checkoutID, status string) (bool, error) {
	tag, err := pool.ExecContext(ctx,
		`UPDATE transactions SET status = $2 WHERE checkout_id = $1`,
		checkoutID, status,
	)
	if err != nil {
		return false, err
	}

	affected, err := tag.RowsAffected()
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}
