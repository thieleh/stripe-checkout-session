# Stripe Checkout Service

This service provides a simple API to create and manage **Stripe Checkout Sessions**, supporting both:

- **Hosted Checkout (default)** – Stripe-hosted payment page.
- **Embedded Checkout (optional)** – Checkout embedded within your frontend app.

It also handles **Stripe webhooks** for `checkout.session.completed` events, using **Stripe Staging API keys** for testing.

---

### 2. Install Dependencies

```bash
   go mod tidy
```
---

### 3. Environment Variables
   Create a `.env` file or export these variables manually:

---

``` bash
export STRIPE_SECRET_API_KEY=sk_test_key
export STRIPE_WEBHOOK_SECRET=whsec_your_webhook_secret  # only needed for CLI webhook testing
export PORT=8080
export ENV=development  # or staging/production
```
- In *development* mode, webhook signature verification is skipped for easier testing.
- In *staging* or *production*, the service validates Stripe signatures.
___

### 4. Run the Service

```bash
   go run main.go
```

   The service will run on:
```bash
   http://localhost:8080
```

---

## API Endpoints
### 1. Create a Checkout Session
   `POST /checkout/session`

Creates a Stripe Checkout Session.
By default, uses Hosted Checkout with a $10 test product.

Hosted Checkout Request:
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

Embedded Checkout Request:
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

`success_url` and `cancel_url` must not be provided for embedded sessions.

For embedded mode, the service automatically sets `redirect_on_completion: "never"`.

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

Fetch session details.

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
  "product_name": "Test Product"
}
```

---

### 3. Stripe Webhook Endpoint
`POST /webhook/stripe`

Handles events from Stripe such as `checkout.session.completed`.

In development, signature verification is skipped (for local/manual testing).

In staging and production, signatures are verified using STRIPE_WEBHOOK_SECRET.

To simulate in development:

```bash
curl -X POST http://localhost:8080/webhook/stripe \
-H "Content-Type: application/json" \
-d '{"type": "checkout.session.completed", "id": "evt_test_123"}'
```
---

## Testing Real Webhooks with Stripe CLI

1. Start the service in staging mode (signature verification ON):

```bash
export ENV=staging
export STRIPE_SECRET_API_KEY=sk_test_key
go run main.go
```
2. Run Stripe CLI in another terminal:

```bash
stripe listen --forward-to localhost:8080/webhook/stripe
```

3. Copy the whsec_... secret that Stripe CLI prints and export it:

```bash
export STRIPE_WEBHOOK_SECRET=whsec_123456
```
4. Restart the service so the secret is loaded:
```bash
go run main.go
```
5. Trigger a test event:

```bash
stripe trigger checkout.session.completed
```

6. You should see this in your logs:

```yaml
Checkout session completed: evt_12345
```
---

### Hosted vs Embedded Checkout
## Hosted Checkout (Default)
- Customers are redirected to a Stripe-hosted payment page.
- Requires success_url and cancel_url for post-payment redirects.
- Lowest effort to integrate (no frontend work required).

## Embedded Checkout
- Runs inside your frontend app via Stripe’s Embedded Checkout SDK.
- Does not use success_url or cancel_url.
- Automatically disables redirects (redirect_on_completion: "never") so you control the post-payment flow.
- Requires additional frontend integration but allows a branded, seamless user experience.

---

## OpenAPI Specification
The API is fully described in:

```bash
   ./docs/openapi.yaml
```

Importing into Insomnia/Postman
1. Go to Import → From File in Insomnia.
2. Select openapi.yaml.
3. Generate endpoints for testing.

