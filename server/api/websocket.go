package api

import (
	"ecpay-server/driver"
	"ecpay-server/protocol"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type WebRequest struct {
	Command string `json:"command"` // "SALE", "REFUND"
	Amount  string `json:"amount"`  // e.g. "100"
	OrderNo string `json:"order_no"`
}

type WebResponse struct {
	Status  string      `json:"status"` // "success", "error", "processing"
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type Handler struct {
	Manager *driver.SerialManager
	mu      sync.Mutex // Ensure one transaction at a time per server instance
}

func NewHandler(manager *driver.SerialManager) *Handler {
	return &Handler{Manager: manager}
}

func (h *Handler) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	defer conn.Close()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var req WebRequest
		if err := json.Unmarshal(msg, &req); err != nil {
			h.sendJSON(conn, "error", "Invalid JSON", nil)
			continue
		}

		go h.handleRequest(conn, req)
	}
}

func (h *Handler) sendJSON(conn *websocket.Conn, status, message string, data interface{}) {
	resp := WebResponse{
		Status:  status,
		Message: message,
		Data:    data,
	}
	conn.WriteJSON(resp)
}

func (h *Handler) handleRequest(conn *websocket.Conn, req WebRequest) {
	// Try to lock for transaction
	if !h.mu.TryLock() {
		h.sendJSON(conn, "error", "POS is busy", nil)
		return
	}
	defer h.mu.Unlock()

	h.sendJSON(conn, "processing", "Initializing transaction...", nil)

	// Build Protocol Request
	var ecpayReq protocol.ECPayRequest

	switch req.Command {
	case "SALE":
		ecpayReq.TransType = "01"
		ecpayReq.HostID = "01"
		ecpayReq.Amount = req.Amount
	case "REFUND":
		ecpayReq.TransType = "02"
		ecpayReq.HostID = "01"
		ecpayReq.Amount = req.Amount
		ecpayReq.OrderNo = req.OrderNo
	case "SETTLEMENT":
		ecpayReq.TransType = "50"
		ecpayReq.HostID = "01"
		ecpayReq.Amount = "0" // Must be zero for settlement
	case "ECHO":
		ecpayReq.TransType = "80"
		ecpayReq.HostID = "01"
	default:
		h.sendJSON(conn, "error", "Unknown Command", nil)
		return
	}

	h.sendJSON(conn, "processing", "Request sent to POS. Waiting for operation...", nil)

	// Execute
	result, err := h.Manager.ExecuteTransaction(ecpayReq)
	if err != nil {
		h.sendJSON(conn, "error", err.Error(), nil)
		return
	}

	// Success
	h.sendJSON(conn, "success", "Transaction Approved", result)
}
