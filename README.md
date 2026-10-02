# maya-gateway

A thin HTTP gateway in front of the [Maya](https://developers.maya.ph/) Checkout API. The gateway
mirrors Maya's upstream paths so a client can call it directly without knowing the upstream host.

## Requirements

- Go 1.27.0
- Maya sandbox API keys from the [Maya Business Manager](https://pbm.paymaya.com/)
- A reachable PostgreSQL server. The gateway connects and pings on startup and exits if it fails.

## Setup

```bash
git clone <repo-url>
cd maya-gateway
go mod download
```

## Environment Variables

Create a `.env` file inside `maya-gateway/`. It is loaded at startup by `godotenv.Load()`, which
reads from the process working directory, so the binary must be run from inside `maya-gateway/`.

| Key | Required | Description |
|---|---|---|
| `MAYA_BASE_URL` | Yes | Maya API host. Sandbox: `https://pg-sandbox.paymaya.com`, production: `https://pg.paymaya.com` |
| `MAYA_PUBLIC_KEY` | Yes | Public API key (`pk-...`). Used for Basic auth on checkout endpoints. |
| `MAYA_SECRET_KEY` | Yes | Secret API key (`sk-...`). Used for Basic auth on Get Checkout. |
| `DB_HOST` | Yes | PostgreSQL host, e.g. `localhost` |
| `DB_PORT` | Yes | PostgreSQL port, e.g. `5432` |
| `DB_USER` | Yes | PostgreSQL role |
| `DB_PASSWORD` | Yes | Password for that role |
| `DB_NAME` | Yes | Database name |

The app exits at startup if `.env` cannot be read, or if it cannot reach the database.

## Run

```bash
cd maya-gateway
go run .
```

The server listens on `:8080`. Open <http://localhost:8080> for a demo page that creates a
checkout and redirects into Maya's hosted payment page.

The pages are embedded into the binary via `go:embed`, so they are served regardless of working
directory. Edits to `web/*.html` require a restart (`go run .`) to take effect.

## Database

`db.Connect` builds a PostgreSQL connection string from the `DB_*` variables and verifies it with a
5-second ping. The gateway exits on failure rather than starting in a degraded state.

There is no schema and no migrations. The connection is established but nothing reads from it yet —
the payment routes do not touch the database, so an outage there will not affect them.

`sslmode=disable` is set explicitly, which suits a local sandbox server with no TLS certificate. Any
deployment where the connection leaves the machine should use `require` or `verify-full` instead.

## Demo Pages

These exist for manual testing. They are not part of the payment API.

| Route | Purpose |
|---|---|
| `GET /` | Checkout page. Creates a checkout and redirects to Maya's hosted page |
| `GET /success` | Maya's `redirectUrl.success` target. Shows the outcome plus live checkout status |
| `GET /failure` | Maya's `redirectUrl.failure` target |
| `GET /cancel` | Maya's `redirectUrl.cancel` target |

The result pages report which URL Maya redirected the browser to, then query Get Checkout to show
the actual `status` and `paymentStatus` from Maya. The `checkoutId` is carried across in
`localStorage`; if it is missing (different browser, cleared storage) the page falls back to a
manual lookup box, which is also the easiest way to re-inspect an earlier or expired checkout.

### Sandbox test cards

The cards listed in Maya's own documentation have expired (the earliest show `05/2021`, the rest
`12/2025`). Current cards from the [Maya mock processor](https://mock-processor-sandbox.paymaya.com/cards):

| Card | Expiry | CVV | Passkey | Result |
|---|---|---|---|---|
| `5204 2404 9566 9629` | 12/2031 | 111 | — | Success |
| `5453 0100 0006 4154` | 12/2030 | 111 | `secbarry1` | Success (3DS challenge) |
| `4917 6100 0000 0000` | 12/2030 | 888 | `paymaya88` | Fails: insufficient funds |

## API

### Create Checkout

```
POST /checkout/v1/checkouts
```

Proxies to `POST {MAYA_BASE_URL}/checkout/v1/checkouts`. Authenticated with HTTP Basic using the
public key as the username and a blank password.

**Request body**

| Field | Type | Required | Description |
|---|---|---|---|
| `totalAmount.value` | number | Yes | Total transaction amount |
| `totalAmount.currency` | string | Yes | ISO 4217 alpha-3, e.g. `PHP` |
| `totalAmount.details` | object | No | Amount breakdown (`subtotal`, `discount`, `serviceCharge`, `shippingFee`, `tax`) |
| `buyer` | object | No | Buyer details. All fields optional unless Kount fraud protection is enabled |
| `buyer.contact` | object | No | `phone` and/or `email` |
| `buyer.billingAddress` | object | No | `line1`, `line2`, `city`, `state`, `zipCode`, `countryCode` |
| `buyer.shippingAddress` | object | No | Billing fields plus `firstName`, `middleName`, `lastName`, `phone`, `email`, `shippingType` (`ST` or `SD`) |
| `items` | array | No | Each item requires `name` and `totalAmount`; optional `code`, `description`, `quantity`, `amount` |
| `redirectUrl` | object | No | `success`, `failure`, `cancel` URLs |
| `requestReferenceNumber` | string | Yes | Merchant reference, 1–36 chars, unique per transaction |
| `metadata` | object | No | `subMerchantRequestReferenceNumber` and `pf` (payment facilitator) |

```bash
curl -X POST http://127.0.0.1:8080/checkout/v1/checkouts \
  -H 'Content-Type: application/json' \
  -d '{
    "totalAmount": { "value": 1000, "currency": "PHP" },
    "buyer": {
      "firstName": "Francis",
      "lastName": "Gonzales",
      "contact": { "email": "maya.juan@mail.com" }
    },
    "items": [
      {
        "name": "Canvas Slip Ons",
        "code": "CVG-096732",
        "quantity": "1",
        "amount": { "value": 1000 },
        "totalAmount": { "value": 1000 }
      }
    ],
    "redirectUrl": {
      "success": "https://www.merchantsite.com/success",
      "failure": "https://www.merchantsite.com/failure",
      "cancel": "https://www.merchantsite.com/cancel"
    },
    "requestReferenceNumber": "5fc10b93-bdbd-4f31-b31d-4575a3785009"
  }'
```

**Response — `200 OK`**

```json
{
  "checkoutId": "14f3571f-7e3d-426f-9f43-64a020c7b48d",
  "redirectUrl": "https://payments-web-sandbox.maya.ph/v2/checkout?id=14f3571f-7e3d-426f-9f43-64a020c7b48d"
}
```

Redirect the buyer to `redirectUrl` to complete payment. Checkout sessions expire one hour after
creation.

**Errors**

| Status | Cause |
|---|---|
| `400` | Malformed JSON, or Maya rejected the payload. The response is `{"error": "<detail>"}` |
| `4xx` / `5xx` from Maya | Relayed verbatim with Maya's status code and error body |
| `502` | The upstream call failed or the response could not be decoded |

### Get Checkout

```
GET /checkout/v1/checkouts/:id
```

Proxies to `GET {MAYA_BASE_URL}/checkout/v1/checkouts/{id}` where `:id` is the `checkoutId` returned
by Create Checkout.

**This endpoint authenticates with the secret key**, unlike Create Checkout which uses the public
key. Sending the public key returns `401` with Maya code `K004`.

```bash
curl http://127.0.0.1:8080/checkout/v1/checkouts/14f3571f-7e3d-426f-9f43-64a020c7b48d
```

**Response — `200 OK`**

```json
{
  "id": "14f3571f-7e3d-426f-9f43-64a020c7b48d",
  "status": "CREATED",
  "paymentStatus": "PENDING_TOKEN",
  "requestReferenceNumber": "5fc10b93-bdbd-4f31-b31d-4575a3785009",
  "totalAmount": { "amount": "1000", "currency": "PHP" },
  "items": [{ "name": "Canvas Slip Ons", "totalAmount": { "value": 1000 } }],
  "refundedAmount": "0",
  "createdAt": "2026-10-01T14:25:22.926Z",
  "updatedAt": "2026-10-01T14:25:22.926Z",
  "expiredAt": "2026-10-01T15:25:22.926Z"
}
```

Note that `totalAmount` comes back as `amount` (a **string**), not `value` (a number) as it does on
the Create Checkout request. `totalAmount.details`, `metadata`, `paymentDetails`, and
`transactionReferenceNumber` are omitted from the response when Maya returns them empty. `items` keep
the same `value` shape used on the request.

**Errors**

| Status | Cause |
|---|---|
| `400` | `:id` is not a UUID v4. Maya code `2553` |
| `404` | No checkout with that ID. Maya code `PY0009` |
| `4xx` / `5xx` from Maya | Relayed verbatim with Maya's status code and error body |
| `502` | The upstream call failed or the response could not be decoded |

## Notes

- Amounts are sent as JSON numbers. Maya's docs show string values in examples (`"1000.00"`), but
  numbers are accepted and avoid floating-point formatting issues.
- Checkout sessions expire 1 hour after creation. A checkout that is never paid shows `status`
  `CREATED` with `paymentStatus` `PENDING_TOKEN`, then drops to `EXPIRED`.
- The gateway implements Create Checkout and Get Checkout only. Webhooks, voids, refunds, and
  captures are not implemented. Get Checkout is a status fallback for when webhooks are not set up;
  it polls a single checkout on demand.
- `GET /payments/v1/payments/{paymentId}` and `GET /payments/v1/payments/{paymentId}/status` are
  Maya's newer payment retrieval endpoints. Both are reachable from the same client, but only
  `/checkout/v1/checkouts/{id}` is proxied today.
- The gateway has no authentication of its own. Get Checkout authenticates upstream with the Maya
  secret key, so anything able to reach the port can retrieve checkout status. Fine for local
  development, not suitable for exposure as-is.
