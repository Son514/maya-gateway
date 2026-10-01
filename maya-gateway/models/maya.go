package models

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Maya struct {
	BaseURL    string
	PublicKey  string
	SecretKey  string
	HTTPClient *http.Client
}

func NewMaya(baseURL, publicKey, secretKey string) *Maya {
	return &Maya{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		PublicKey:  publicKey,
		SecretKey:  secretKey,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

type AmountDetails struct {
	Subtotal      string `json:"subtotal,omitempty"`
	Discount      string `json:"discount,omitempty"`
	ServiceCharge string `json:"serviceCharge,omitempty"`
	ShippingFee   string `json:"shippingFee,omitempty"`
	Tax           string `json:"tax,omitempty"`
}

type Amount struct {
	Value   float64        `json:"value"`
	Details *AmountDetails `json:"details,omitempty"`
}

type TotalAmount struct {
	Value    float64        `json:"value"`
	Currency string         `json:"currency"`
	Details  *AmountDetails `json:"details,omitempty"`
}

type Contact struct {
	Phone string `json:"phone,omitempty"`
	Email string `json:"email,omitempty"`
}

type Address struct {
	Line1       string `json:"line1,omitempty"`
	Line2       string `json:"line2,omitempty"`
	City        string `json:"city,omitempty"`
	State       string `json:"state,omitempty"`
	ZipCode     string `json:"zipCode,omitempty"`
	CountryCode string `json:"countryCode,omitempty"`
}

type ShippingAddress struct {
	Address
	FirstName    string `json:"firstName,omitempty"`
	MiddleName   string `json:"middleName,omitempty"`
	LastName     string `json:"lastName,omitempty"`
	Phone        string `json:"phone,omitempty"`
	Email        string `json:"email,omitempty"`
	ShippingType string `json:"shippingType,omitempty"`
}

type Buyer struct {
	FirstName       string           `json:"firstName,omitempty"`
	MiddleName      string           `json:"middleName,omitempty"`
	LastName        string           `json:"lastName,omitempty"`
	Birthday        string           `json:"birthday,omitempty"`
	CustomerSince   string           `json:"customerSince,omitempty"`
	Sex             string           `json:"sex,omitempty"`
	Contact         *Contact         `json:"contact,omitempty"`
	BillingAddress  *Address         `json:"billingAddress,omitempty"`
	ShippingAddress *ShippingAddress `json:"shippingAddress,omitempty"`
}

type Item struct {
	Name        string  `json:"name"`
	Code        string  `json:"code,omitempty"`
	Description string  `json:"description,omitempty"`
	Quantity    string  `json:"quantity,omitempty"`
	Amount      *Amount `json:"amount,omitempty"`
	TotalAmount *Amount `json:"totalAmount"`
}

type RedirectURL struct {
	Success string `json:"success,omitempty"`
	Failure string `json:"failure,omitempty"`
	Cancel  string `json:"cancel,omitempty"`
}

type PaymentFacilitator struct {
	SMI          string `json:"smi,omitempty"`
	SMN          string `json:"smn,omitempty"`
	MCI          string `json:"mci,omitempty"`
	MPC          string `json:"mpc,omitempty"`
	MCO          string `json:"mco,omitempty"`
	MST          string `json:"mst,omitempty"`
	MCC          string `json:"mcc,omitempty"`
	PostalCode   string `json:"postalCode,omitempty"`
	ContactNo    string `json:"contactNo,omitempty"`
	State        string `json:"state,omitempty"`
	AddressLine1 string `json:"addressLine1,omitempty"`
}

type Metadata struct {
	SubMerchantRequestReferenceNumber string              `json:"subMerchantRequestReferenceNumber,omitempty"`
	PF                                *PaymentFacilitator `json:"pf,omitempty"`
}

type CreateCheckoutRequest struct {
	TotalAmount            *TotalAmount `json:"totalAmount"`
	Buyer                  *Buyer       `json:"buyer,omitempty"`
	Items                  []Item       `json:"items,omitempty"`
	RedirectURL            *RedirectURL `json:"redirectUrl,omitempty"`
	RequestReferenceNumber string       `json:"requestReferenceNumber"`
	Metadata               *Metadata    `json:"metadata,omitempty"`
}

type CheckoutResponse struct {
	CheckoutID  string `json:"checkoutId"`
	RedirectURL string `json:"redirectUrl"`
}

type CheckoutTotalAmount struct {
	Amount   string         `json:"amount"`
	Currency string         `json:"currency"`
	Details  *AmountDetails `json:"details,omitempty"`
}

type CheckoutStatusResponse struct {
	ID                         string               `json:"id"`
	Status                     string               `json:"status"`
	PaymentStatus              string               `json:"paymentStatus"`
	RequestReferenceNumber     string               `json:"requestReferenceNumber"`
	TransactionReferenceNumber string               `json:"transactionReferenceNumber,omitempty"`
	ReceiptNumber              string               `json:"receiptNumber,omitempty"`
	TotalAmount                *CheckoutTotalAmount `json:"totalAmount,omitempty"`
	Items                      []Item               `json:"items,omitempty"`
	Metadata                   *Metadata            `json:"metadata,omitempty"`
	PaymentDetails             map[string]any       `json:"paymentDetails,omitempty"`
	RefundedAmount             string               `json:"refundedAmount,omitempty"`
	CreatedAt                  string               `json:"createdAt"`
	UpdatedAt                  string               `json:"updatedAt"`
	ExpiredAt                  string               `json:"expiredAt,omitempty"`
}

type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("maya: unexpected status %d: %s", e.StatusCode, e.Body)
}

func (m *Maya) do(ctx context.Context, method, path, key string, body io.Reader) ([]byte, error) {
	httpReq, err := http.NewRequestWithContext(ctx, method, m.BaseURL+path, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.SetBasicAuth(key, "")

	res, err := m.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return nil, &APIError{StatusCode: res.StatusCode, Body: string(resBody)}
	}

	return resBody, nil
}

// Returns Maya's response (including checkoutId and redirectUrl) to the caller.
func (m *Maya) CreateCheckout(ctx context.Context, req CreateCheckoutRequest) (*CheckoutResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	resBody, err := m.do(ctx, http.MethodPost, "/checkout/v1/checkouts", m.PublicKey, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	var checkout CheckoutResponse
	if err := json.Unmarshal(resBody, &checkout); err != nil {
		return nil, err
	}

	return &checkout, nil
}

// Returns Maya's checkout status for the given checkoutId to the caller.
func (m *Maya) GetCheckout(ctx context.Context, id string) (*CheckoutStatusResponse, error) {
	resBody, err := m.do(ctx, http.MethodGet, "/checkout/v1/checkouts/"+url.PathEscape(id), m.SecretKey, nil)
	if err != nil {
		return nil, err
	}

	var checkout CheckoutStatusResponse
	if err := json.Unmarshal(resBody, &checkout); err != nil {
		return nil, err
	}

	return &checkout, nil
}
