package payment

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sikozonpc/cinema/internal/auth"
	"github.com/sikozonpc/cinema/internal/utils"
)

type Handler struct {
	keyID      string
	keySecret  string
	repository *MongoRepository
	onCaptured func(context.Context, VerificationRequest) (string, error)
	httpClient *http.Client
}

func NewHandler(keyID, keySecret string, repository *MongoRepository) *Handler {
	return &Handler{
		keyID:      keyID,
		keySecret:  keySecret,
		repository: repository,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (h *Handler) SetCapturedCallback(callback func(context.Context, VerificationRequest) (string, error)) {
	h.onCaptured = callback
}

type razorpayOrderResponse struct {
	ID       string `json:"id"`
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
	Status   string `json:"status"`
}

func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	if h.keyID == "" || h.keySecret == "" {
		utils.WriteError(w, http.StatusServiceUnavailable, "Razorpay is not configured")
		return
	}
	var req OrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.BookingID == "" || req.Amount <= 0 {
		utils.WriteError(w, http.StatusBadRequest, "booking_id and positive amount are required")
		return
	}
	if req.Currency != "" && req.Currency != "INR" {
		utils.WriteError(w, http.StatusBadRequest, "only INR payments are supported")
		return
	}
	req.Currency = "INR"

	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		utils.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	payload := map[string]any{
		"amount":   int64(math.Round(req.Amount * 100)),
		"currency": req.Currency,
		"receipt":  req.BookingID,
		"notes": map[string]string{
			"booking_id": req.BookingID,
			"user_id":    claims.UserID,
		},
	}
	body, _ := json.Marshal(payload)
	httpReq, err := http.NewRequest(http.MethodPost, "https://api.razorpay.com/v1/orders", bytes.NewReader(body))
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "could not create payment order")
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.SetBasicAuth(h.keyID, h.keySecret)

	response, err := h.httpClient.Do(httpReq)
	if err != nil {
		utils.WriteError(w, http.StatusBadGateway, "payment provider unavailable")
		return
	}
	defer response.Body.Close()
	responseBody, _ := io.ReadAll(response.Body)
	if response.StatusCode >= 300 {
		utils.WriteError(w, http.StatusBadGateway, "payment provider rejected the order")
		return
	}
	var order razorpayOrderResponse
	if err := json.Unmarshal(responseBody, &order); err != nil {
		utils.WriteError(w, http.StatusBadGateway, "invalid payment provider response")
		return
	}
	if order.ID == "" || order.Amount != int64(math.Round(req.Amount*100)) || order.Currency != req.Currency {
		utils.WriteError(w, http.StatusBadGateway, "payment provider returned an unexpected order")
		return
	}

	record := Record{
		ID:           order.ID,
		BookingID:    req.BookingID,
		UserID:       claims.UserID,
		Provider:     "razorpay",
		Transaction:  order.ID,
		Amount:       req.Amount,
		Currency:     order.Currency,
		Status:       "created",
		RefundStatus: "none",
		Timestamp:    time.Now().UTC(),
	}
	if err := h.repository.Save(r.Context(), record); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "could not persist payment order")
		return
	}
	utils.WriteJSON(w, http.StatusCreated, map[string]any{
		"order_id": order.ID,
		"amount":   order.Amount,
		"currency": order.Currency,
		"key_id":   h.keyID,
	})
}

type VerificationRequest struct {
	RazorpayOrderID   string `json:"razorpay_order_id"`
	RazorpayPaymentID string `json:"razorpay_payment_id"`
	RazorpaySignature string `json:"razorpay_signature"`
	SessionID         string `json:"session_id"`
}

type razorpayPaymentResponse struct {
	ID       string `json:"id"`
	OrderID  string `json:"order_id"`
	Amount   int64  `json:"amount"`
	Status   string `json:"status"`
	Currency string `json:"currency"`
}

func (h *Handler) Verify(w http.ResponseWriter, r *http.Request) {
	if h.keySecret == "" {
		utils.WriteError(w, http.StatusServiceUnavailable, "Razorpay is not configured")
		return
	}
	var req VerificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	payload := req.RazorpayOrderID + "|" + req.RazorpayPaymentID
	mac := hmac.New(sha256.New, []byte(h.keySecret))
	_, _ = mac.Write([]byte(payload))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(req.RazorpaySignature)) {
		utils.WriteError(w, http.StatusBadRequest, "invalid payment signature")
		return
	}
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		utils.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	if req.RazorpayOrderID == "" || req.RazorpayPaymentID == "" || req.RazorpaySignature == "" || req.SessionID == "" {
		utils.WriteError(w, http.StatusBadRequest, "payment identifiers and session_id are required")
		return
	}
	if h.onCaptured == nil {
		utils.WriteError(w, http.StatusServiceUnavailable, "booking confirmation is unavailable")
		return
	}
	orderRecord, err := h.repository.FindOrder(r.Context(), req.RazorpayOrderID, req.SessionID, claims.UserID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "could not verify payment order")
		return
	}
	if orderRecord.ID == "" {
		utils.WriteError(w, http.StatusNotFound, "payment order was not found for this booking")
		return
	}
	if orderRecord.Status != "created" && orderRecord.Status != "captured" {
		utils.WriteError(w, http.StatusConflict, "payment order is not payable")
		return
	}
	if orderRecord.Status == "captured" && orderRecord.Transaction != req.RazorpayPaymentID {
		utils.WriteError(w, http.StatusConflict, "payment order already has a different captured payment")
		return
	}
	payment, err := h.fetchPayment(r.Context(), req.RazorpayPaymentID)
	if err != nil {
		utils.WriteError(w, http.StatusBadGateway, "could not confirm payment status with Razorpay")
		return
	}
	if payment.ID != req.RazorpayPaymentID ||
		payment.OrderID != req.RazorpayOrderID ||
		payment.Amount != int64(math.Round(orderRecord.Amount*100)) ||
		payment.Currency != orderRecord.Currency ||
		payment.Status != "captured" {
		utils.WriteError(w, http.StatusConflict, "Razorpay has not captured the expected payment")
		return
	}
	record := Record{
		ID:           orderRecord.ID,
		BookingID:    orderRecord.BookingID,
		UserID:       orderRecord.UserID,
		Provider:     orderRecord.Provider,
		Transaction:  req.RazorpayPaymentID,
		Amount:       orderRecord.Amount,
		Currency:     orderRecord.Currency,
		Status:       "captured",
		RefundStatus: "none",
		Timestamp:    time.Now().UTC(),
	}
	ticketID := ""
	if h.onCaptured != nil {
		issuedTicketID, err := h.onCaptured(r.Context(), req)
		if err != nil {
			utils.WriteError(w, http.StatusConflict, "payment verified but seat confirmation failed")
			return
		}
		ticketID = issuedTicketID
	}
	if err := h.repository.UpdateOrder(r.Context(), req.RazorpayOrderID, record); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "seat confirmed but could not persist verified payment")
		return
	}
	utils.WriteJSON(w, http.StatusOK, map[string]string{"status": "captured", "ticket_id": ticketID})
}

func (h *Handler) fetchPayment(ctx context.Context, paymentID string) (razorpayPaymentResponse, error) {
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		"https://api.razorpay.com/v1/payments/"+url.PathEscape(paymentID),
		nil,
	)
	if err != nil {
		return razorpayPaymentResponse{}, err
	}
	request.SetBasicAuth(h.keyID, h.keySecret)
	response, err := h.httpClient.Do(request)
	if err != nil {
		return razorpayPaymentResponse{}, err
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusMultipleChoices {
		return razorpayPaymentResponse{}, fmt.Errorf("Razorpay returned status %d", response.StatusCode)
	}
	var payment razorpayPaymentResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payment); err != nil {
		return razorpayPaymentResponse{}, err
	}
	return payment, nil
}

func (h *Handler) verifyWebhook(raw []byte, signature string) bool {
	mac := hmac.New(sha256.New, []byte(h.keySecret))
	_, _ = mac.Write(raw)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(strings.TrimPrefix(signature, "sha256=")))
}

func (h *Handler) RazorpayWebhook(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "invalid webhook body")
		return
	}
	if h.keySecret == "" || !h.verifyWebhook(raw, r.Header.Get("X-Razorpay-Signature")) {
		utils.WriteError(w, http.StatusUnauthorized, "invalid webhook signature")
		return
	}
	utils.WriteJSON(w, http.StatusOK, map[string]string{"status": fmt.Sprintf("accepted")})
}
