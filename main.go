package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/account"
	"github.com/stripe/stripe-go/v81/charge"
	"github.com/stripe/stripe-go/v81/checkout/session"
	"github.com/stripe/stripe-go/v81/paymentintent"
	"github.com/stripe/stripe-go/v81/webhook"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

func main() {
	godotenv.Load()

	stripe.Key = os.Getenv("STRIPE_SECRET_API_KEY")
	if stripe.Key == "" {
		log.Fatal("Missing STRIPE_SECRET_API_KEY env variable")
	}
	log.Printf("Stripe key loaded (starts with): %s...", stripe.Key[:8])

	r := mux.NewRouter()
	r.HandleFunc("/checkout/session", CreateCheckoutSession).Methods("POST")
	r.HandleFunc("/checkout/session/{id}", GetCheckoutSession).Methods("GET")
	r.HandleFunc("/checkout/session/{id}", UpdateCheckoutSession).Methods("PATCH")
	r.HandleFunc("/payment_intents/{id}", GetPaymentIntent).Methods("GET")
	r.HandleFunc("/charges/{id}", GetCharge).Methods("GET")
	r.HandleFunc("/account", GetAccount).Methods("GET")
	r.HandleFunc("/webhook/stripe", HandleStripeWebhook).Methods("POST")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server running on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

type CreateSessionRequest struct {
	LineItems     []LineItem `json:"line_items"`
	SuccessURL    string     `json:"success_url"`
	CancelURL     string     `json:"cancel_url"`
	Mode          string     `json:"mode"`
	CustomerEmail string     `json:"customer_email"`
	UIMode        string     `json:"ui_mode"`
}

type LineItem struct {
	Price    string `json:"price"`
	Quantity int64  `json:"quantity"`
}

func CreateCheckoutSession(w http.ResponseWriter, r *http.Request) {
	stripe.Key = os.Getenv("STRIPE_SECRET_API_KEY")

	var req CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if len(req.LineItems) == 0 {
		// Using a default Price ID so we avoid resource_missing
		req.LineItems = []LineItem{{Price: "price_1RooOjFN9iVxHY4vNARpSFFc", Quantity: 1}}
	}
	if req.UIMode == "" {
		req.UIMode = "hosted"
	}

	params := &stripe.CheckoutSessionParams{
		Mode:       stripe.String(req.Mode),
		UIMode:     stripe.String(req.UIMode),
		SuccessURL: stripe.String(req.SuccessURL),
		CancelURL:  stripe.String(req.CancelURL),
	}
	if req.CustomerEmail != "" {
		params.CustomerEmail = stripe.String(req.CustomerEmail)
	}
	for _, li := range req.LineItems {
		params.LineItems = append(params.LineItems, &stripe.CheckoutSessionLineItemParams{
			Price:    stripe.String(li.Price),
			Quantity: stripe.Int64(li.Quantity),
		})
	}

	log.Printf("[REQUEST] Creating Checkout Session (Mode: %s, UIMode: %s)", req.Mode, req.UIMode)
	s, err := session.New(params)
	if err != nil {
		log.Printf("[ERROR] Stripe session.New: %v", err)
		http.Error(w, fmt.Sprintf("Stripe API error: %v", err), http.StatusBadRequest)
		return
	}

	log.Printf("[SUCCESS] Created Checkout Session: %s (URL: %s)", s.ID, s.URL)
	resp := map[string]string{"id": s.ID, "url": s.URL}
	json.NewEncoder(w).Encode(resp)
}

func GetCheckoutSession(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	log.Printf("[REQUEST] Fetching Checkout Session: %s", id)

	s, err := session.Get(id, nil)
	if err != nil {
		log.Printf("[ERROR] session.Get: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("[SUCCESS] Fetched Checkout Session: %s (Status: %s, Amount: %d)", s.ID, s.Status, s.AmountTotal)
	resp := map[string]interface{}{"id": s.ID, "status": s.Status, "amount": s.AmountTotal}
	json.NewEncoder(w).Encode(resp)
}

func UpdateCheckoutSession(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	log.Printf("[REQUEST] Updating Checkout Session: %s", id)

	var payload struct {
		NewPrice     string                 `json:"new_price"`
		Shipping     map[string]interface{} `json:"shipping_address"`
		TransferData map[string]interface{} `json:"transfer_data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// NOTE: Stripe Checkout Sessions are limited in updates.
	// Typically you'd cancel + recreate or update PaymentIntent.
	params := &stripe.CheckoutSessionParams{}
	if payload.NewPrice != "" {
		params.LineItems = []*stripe.CheckoutSessionLineItemParams{
			{Price: stripe.String(payload.NewPrice), Quantity: stripe.Int64(1)},
		}
	}
	log.Printf("[INFO] Attempting update with price=%s transferData=%v shipping=%v", payload.NewPrice, payload.TransferData, payload.Shipping)

	s, err := session.Update(id, params)
	if err != nil {
		log.Printf("[ERROR] session.Update: %v", err)
		http.Error(w, fmt.Sprintf("Stripe API error: %v", err), http.StatusBadRequest)
		return
	}

	log.Printf("[SUCCESS] Updated Checkout Session: %s", s.ID)
	resp := map[string]interface{}{"id": s.ID, "updated": true}
	json.NewEncoder(w).Encode(resp)
}

func GetPaymentIntent(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	log.Printf("[REQUEST] Fetching PaymentIntent: %s", id)

	pi, err := paymentintent.Get(id, nil)
	if err != nil {
		log.Printf("[ERROR] paymentintent.Get: %v", err)
		http.Error(w, fmt.Sprintf("Stripe API error: %v", err), http.StatusBadRequest)
		return
	}

	log.Printf("[SUCCESS] PaymentIntent: %s (Status: %s, Amount: %d)", pi.ID, pi.Status, pi.Amount)
	json.NewEncoder(w).Encode(pi)
}

func GetCharge(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	log.Printf("[REQUEST] Fetching Charge: %s", id)

	ch, err := charge.Get(id, nil)
	if err != nil {
		log.Printf("[ERROR] charge.Get: %v", err)
		http.Error(w, fmt.Sprintf("Stripe API error: %v", err), http.StatusBadRequest)
		return
	}

	log.Printf("[SUCCESS] Charge: %s (Amount: %d, Paid: %t)", ch.ID, ch.Amount, ch.Paid)
	json.NewEncoder(w).Encode(ch)
}

func GetAccount(w http.ResponseWriter, r *http.Request) {
	log.Printf("[REQUEST] Fetching Stripe Account")

	acct, err := account.Get()
	if err != nil {
		log.Printf("[ERROR] account.Get: %v", err)
		http.Error(w, fmt.Sprintf("Stripe API error: %v", err), http.StatusBadRequest)
		return
	}

	log.Printf("[SUCCESS] Stripe Account: %s", acct.ID)
	json.NewEncoder(w).Encode(acct)
}

func HandleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	payload, _ := ioutil.ReadAll(r.Body)
	sig := r.Header.Get("Stripe-Signature")

	endpointSecret := os.Getenv("STRIPE_WEBHOOK_SECRET")
	if endpointSecret == "" {
		log.Printf("[WEBHOOK] Missing STRIPE_WEBHOOK_SECRET")
		http.Error(w, "webhook secret not set", http.StatusInternalServerError)
		return
	}

	event, err := webhook.ConstructEventWithOptions(
		payload,
		sig,
		endpointSecret,
		webhook.ConstructEventOptions{IgnoreAPIVersionMismatch: true},
	)
	if err != nil {
		log.Printf("[WEBHOOK] Signature verification failed: %v", err)
		http.Error(w, "signature verification failed", http.StatusBadRequest)
		return
	}

	log.Printf("[WEBHOOK] Event received: %s", event.Type)

	switch event.Type {
	case "checkout.session.completed":
		var session stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &session); err == nil {
			log.Printf("[WEBHOOK] Checkout session completed: ID=%s, Amount=%d, Email=%s", session.ID, session.AmountTotal, session.CustomerEmail)
		}
	case "payment_intent.succeeded":
		var pi stripe.PaymentIntent
		if err := json.Unmarshal(event.Data.Raw, &pi); err == nil {
			log.Printf("[WEBHOOK] PaymentIntent succeeded: ID=%s, Amount=%d", pi.ID, pi.Amount)
		}
	case "charge.succeeded":
		var ch stripe.Charge
		if err := json.Unmarshal(event.Data.Raw, &ch); err == nil {
			log.Printf("[WEBHOOK] Charge succeeded: ID=%s, Amount=%d, Paid=%t", ch.ID, ch.Amount, ch.Paid)
		}
	default:
		log.Printf("[WEBHOOK] Unhandled event type: %s", event.Type)
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ok")
}
