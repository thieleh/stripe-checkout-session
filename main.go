package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/checkout/session"
	"github.com/stripe/stripe-go/v81/webhook"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

type CreateSessionRequest struct {
	Amount        int64  `json:"amount"`         // Amount in cents, defaults to 1000 ($10)
	Currency      string `json:"currency"`       // Currency, defaults to "usd"
	ProductName   string `json:"product_name"`   // Defaults to "Test Product"
	SuccessURL    string `json:"success_url"`    // Required
	CancelURL     string `json:"cancel_url"`     // Required
	Mode          string `json:"mode"`           // "payment" (default) or "subscription"
	CustomerEmail string `json:"customer_email"` // Optional
	UIMode        string `json:"ui_mode"`        // "hosted" (default) or "embedded"
}

func main() {
	// Load environment variables
	_ = godotenv.Load()

	stripe.Key = os.Getenv("STRIPE_SECRET_API_KEY")
	if stripe.Key == "" {
		log.Fatal("Missing STRIPE_SECRET_API_KEY env variable")
	}
	log.Printf("Stripe key loaded (starts with): %s...", stripe.Key[:8])

	// Setup routes
	r := mux.NewRouter()
	r.HandleFunc("/checkout/session", CreateCheckoutSession).Methods("POST")
	r.HandleFunc("/checkout/session/{id}", GetCheckoutSession).Methods("GET")
	r.HandleFunc("/webhook/stripe", HandleStripeWebhook).Methods("POST")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server running on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

// CreateCheckoutSession creates a new Stripe Checkout Session.
func CreateCheckoutSession(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid JSON"}`, http.StatusBadRequest)
		return
	}

	if req.Amount <= 0 {
		req.Amount = 1000 // $10 default
	}
	if req.Currency == "" {
		req.Currency = "usd"
	}
	if req.ProductName == "" {
		req.ProductName = "Test Product"
	}
	if req.Mode == "" {
		req.Mode = string(stripe.CheckoutSessionModePayment)
	}
	if req.UIMode == "" {
		req.UIMode = string(stripe.CheckoutSessionUIModeHosted)
	}

	uiMode := stripe.CheckoutSessionUIModeHosted
	if req.UIMode == string(stripe.CheckoutSessionUIModeEmbedded) {
		uiMode = stripe.CheckoutSessionUIModeEmbedded
	}

	// Build session params
	params := &stripe.CheckoutSessionParams{
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency: stripe.String(req.Currency),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String(req.ProductName),
					},
					UnitAmount: stripe.Int64(req.Amount),
				},
				Quantity: stripe.Int64(1),
			},
		},
		Mode:   stripe.String(req.Mode),
		UIMode: stripe.String(string(uiMode)),
	}

	// Only include SuccessURL and CancelURL for hosted checkout
	if uiMode == stripe.CheckoutSessionUIModeEmbedded {
		params.RedirectOnCompletion = stripe.String("never")
	} else {
		params.SuccessURL = stripe.String(req.SuccessURL)
		params.CancelURL = stripe.String(req.CancelURL)
	}

	// Only add customer email if valid
	if req.CustomerEmail != "" && strings.Contains(req.CustomerEmail, "@") {
		params.CustomerEmail = stripe.String(req.CustomerEmail)
	}

	s, err := session.New(params)
	if err != nil {
		log.Printf("Stripe error: %+v\n", err)
		http.Error(w, fmt.Sprintf(`{"error": "Stripe API error: %v"}`, err), http.StatusBadRequest)
		return
	}

	resp := map[string]string{"id": s.ID, "url": s.URL}
	json.NewEncoder(w).Encode(resp)
}

// GetCheckoutSession retrieves the details of a Checkout Session.
func GetCheckoutSession(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := mux.Vars(r)["id"]

	s, err := session.Get(id, nil)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%v"}`, err), http.StatusInternalServerError)
		return
	}

	resp := map[string]interface{}{
		"id":     s.ID,
		"status": s.Status,
		"amount": s.AmountTotal,
	}
	json.NewEncoder(w).Encode(resp)
}

func HandleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, `{"error": "read error"}`, http.StatusBadRequest)
		return
	}

	endpointSecret := os.Getenv("STRIPE_WEBHOOK_SECRET")

	if os.Getenv("ENV") == "development" || endpointSecret == "" {
		log.Println("Skipping signature verification (dev mode)")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
		return
	}

	event, err := webhook.ConstructEvent(payload, r.Header.Get("Stripe-Signature"), endpointSecret)
	if err != nil {
		http.Error(w, `{"error": "signature verification failed"}`, http.StatusBadRequest)
		return
	}

	if event.Type == "checkout.session.completed" {
		log.Printf("Checkout session completed: %s", event.ID)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}
