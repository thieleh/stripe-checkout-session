# Stripe Checkout Service

This service provides a simple API to create, manage, and track **Stripe Checkout Sessions**, supporting both:

- **Hosted Checkout (default)** – Stripe-hosted payment page.
- **Embedded Checkout (optional)** – Checkout embedded within your frontend app.

It also handles **Stripe webhooks** (like `checkout.session.completed`) and can fetch related **Payment Intents**, **Charges**, and **Stripe Account info** for debugging or integration.

All testing uses **Stripe Staging API keys**.

---

## 1. Install Dependencies

```bash
go mod tidy
brew install stripe/stripe-cli/stripe
```

---

## 2. Environment Variables

Create a `.env` file or export these variables manually:

```bash
export STRIPE_SECRET_API_KEY=sk_test_key
export STRIPE_WEBHOOK_SECRET=whsec_your_webhook_secret  # only needed for CLI webhook testing
export PORT=8080
export ENV=development  # or staging/production
```

- In **development mode**, webhook signature verification is skipped (for easy Insomnia/cURL testing).
- In **staging** or **production**, the service validates Stripe signatures to ensure security.

---

## 3. Run the Service

```bash
go run main.go
```

The service will be available at:

```
http://localhost:8080
```

---

## API Endpoints

### 1. Create a Checkout Session
`POST /checkout/session`

Creates a new Checkout Session.

#### Hosted Checkout (Default):
```bash
curl -X POST http://localhost:8080/checkout/session \
-H "Content-Type: application/json" \
-d '{
  "amount": 1000,
  "currency": "usd",
  "product_name": "Test Product",
  "success_url": "https://example.org/success",
  "cancel_url": "https://example.org/cancel",
  "customer_email": "test@example.com"
}'
```

#### Embedded Checkout:
```bash
curl -X POST http://localhost:8080/checkout/session \
-H "Content-Type: application/json" \
-d '{
  "amount": 1000,
  "currency": "usd",
  "product_name": "Test Product",
  "ui_mode": "embedded",
  "customer_email": "test@example.com"
}'
```

- Embedded mode **does not support `success_url` or `cancel_url`**.
- It automatically disables redirects by setting `redirect_on_completion: "never"`.

Response Example:
```json
{
  "id": "cs_test_12345",
  "url": "https://checkout.stripe.com/pay/cs_test_12345"
}
```

---

### 2. Retrieve a Checkout Session
`GET /checkout/session/{session_id}`

Fetch session details, along with related `payment_intent` and `charge` IDs for further testing.

```bash
curl http://localhost:8080/checkout/session/cs_test_12345
```

Response Example:
```json
{
  "id": "cs_test_12345",
  "status": "open",
  "amount": 1000,
  "currency": "usd",
  "payment_intent": "pi_123",
  "charges": ["ch_123"]
}
```

---

### 3. Update a Checkout Session (Limited)
`PATCH /checkout/session/{session_id}`

Stripe only allows **limited updates** to Checkout Sessions (`metadata`, shipping options).  
Line items or prices **cannot be changed**. For new prices, you must create a new session.

Example:
```bash
curl -X PATCH http://localhost:8080/checkout/session/cs_test_12345 \
-H "Content-Type: application/json" \
-d '{
  "metadata": {"order_id": "12345"},
  "shipping_address_collection": {"allowed_countries": ["US"]}
}'
```

---

### 4. Fetch Related Stripe Data

#### a) Fetch Payment Intent
`GET /payment_intents/{payment_intent_id}`

```bash
curl http://localhost:8080/payment_intents/pi_123
```

#### b) Fetch Charge
`GET /charges/{charge_id}`

```bash
curl http://localhost:8080/charges/ch_123
```

#### c) Fetch Stripe Account Info
`GET /account`

```bash
curl http://localhost:8080/account
```

---

### 5. Stripe Webhook Endpoint
`POST /webhook/stripe`

Handles events like `checkout.session.completed`.

- In **development mode**, skips signature verification and logs "DEV mode".
- In **staging/production**, validates signatures using `STRIPE_WEBHOOK_SECRET`.

Testing locally:
```bash
curl -X POST http://localhost:8080/webhook/stripe \
-H "Content-Type: application/json" \
-d '{"type": "checkout.session.completed", "id": "evt_test_123"}'
```

---

## Testing Real Webhooks with Stripe CLI

1. Start the service in **staging mode**:
```bash
export ENV=staging
export STRIPE_SECRET_API_KEY=sk_test_key
go run main.go
```

2. Run Stripe CLI:
```bash
stripe listen --forward-to localhost:8080/webhook/stripe
```

3. Copy the webhook secret it prints and export it:
```bash
export STRIPE_WEBHOOK_SECRET=whsec_123456
```

4. Restart the service:
```bash
go run main.go
```

5. Trigger a test event:
```bash
stripe trigger checkout.session.completed --override price=price_123
```

Logs will include:
```
[WEBHOOK] Checkout session completed (Session ID: cs_test_123, Amount: 1000, Email: test@example.com)
```

---

## Hosted vs Embedded Checkout

### Hosted Checkout (Default)
- Redirects customers to a Stripe-hosted payment page.
- Requires `success_url` and `cancel_url`.
- Quickest and simplest to integrate (no frontend coding).

### Embedded Checkout
- Runs **inside your app** via Stripe’s Embedded Checkout SDK.
- Does **not use `success_url` or `cancel_url`**.
- Automatically disables redirects so you can control post-payment behavior.
- Requires more frontend integration but allows a **branded** experience.

---

## OpenAPI Specification

The API is fully described in:
```
./docs/openapi.yaml
```

### To Import into Insomnia:
1. Open Insomnia → **Import → From File**.
2. Select `openapi.yaml`.
3. Endpoints will be auto-generated for testing.
