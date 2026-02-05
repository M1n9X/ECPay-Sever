package api

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"tcb-server/driver"
	"tcb-server/protocol"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type WebRequest struct {
	Command   string            `json:"command"` // "SALE", "REFUND", "SETTLEMENT", "GETPAN", "COMPLETE", "TERMINATE", "INQUIRY", "TRANSACT", "STATUS", "ABORT", "RECONNECT"
	Amount    string            `json:"amount"`
	OrderNo   string            `json:"order_no"`
	HostID    string            `json:"host_id"`
	TransType string            `json:"trans_type"`
	Fields    map[string]string `json:"fields"`
}

type WebResponse struct {
	Status      string      `json:"status"` // "success", "error", "processing", "status_update"
	Message     string      `json:"message"`
	CommandType string      `json:"command_type"` // "transaction", "control", "status"
	Data        interface{} `json:"data,omitempty"`
}

type Handler struct {
	Manager *driver.SerialManager
	mu      sync.Mutex // Ensure one transaction at a time per server instance

	// Connected clients for broadcasting
	clients   map[*websocket.Conn]bool
	clientsMu sync.RWMutex

	// Status broadcast ticker
	broadcastTicker *time.Ticker
	stopBroadcast   chan struct{}

	// Idempotency cache (business layer)
	idemMu    sync.Mutex
	idemCache map[string]idemRecord
}

func NewHandler(manager *driver.SerialManager) *Handler {
	h := &Handler{
		Manager:       manager,
		clients:       make(map[*websocket.Conn]bool),
		stopBroadcast: make(chan struct{}),
		idemCache:     make(map[string]idemRecord),
	}

	// Set up state change callback
	manager.SetStateCallback(func(info driver.StatusInfo) {
		h.broadcastStatus(info)
	})

	// Start periodic status broadcast (every 1s during active transactions)
	h.broadcastTicker = time.NewTicker(1 * time.Second)
	go h.periodicBroadcast()

	return h
}

// periodicBroadcast sends status updates every second during active transactions
func (h *Handler) periodicBroadcast() {
	for {
		select {
		case <-h.broadcastTicker.C:
			status := h.Manager.GetStatus()
			if status.State != "IDLE" {
				h.broadcastStatus(status)
			}
		case <-h.stopBroadcast:
			h.broadcastTicker.Stop()
			return
		}
	}
}

// broadcastStatus sends status to all connected clients
func (h *Handler) broadcastStatus(info driver.StatusInfo) {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()

	resp := WebResponse{
		Status:  "status_update",
		Message: info.Message,
		Data:    info,
	}

	for conn := range h.clients {
		if err := conn.WriteJSON(resp); err != nil {
			log.Printf("Broadcast error: %v", err)
		}
	}
}

// addClient registers a new client
func (h *Handler) addClient(conn *websocket.Conn) {
	h.clientsMu.Lock()
	defer h.clientsMu.Unlock()
	h.clients[conn] = true
}

// removeClient unregisters a client
func (h *Handler) removeClient(conn *websocket.Conn) {
	h.clientsMu.Lock()
	defer h.clientsMu.Unlock()
	delete(h.clients, conn)
}

func (h *Handler) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	defer func() {
		h.removeClient(conn)
		conn.Close()
	}()

	// Register client
	h.addClient(conn)

	// Send initial status
	status := h.Manager.GetStatus()
	h.sendStatus(conn, status.Message, status)

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var req WebRequest
		if err := json.Unmarshal(msg, &req); err != nil {
			h.sendControl(conn, "error", "Invalid JSON", nil)
			continue
		}

		// Handle different commands
		switch req.Command {
		case "STATUS":
			status := h.Manager.GetStatus()
			h.sendStatus(conn, status.Message, status)
		case "ABORT":
			if h.Manager.AbortTransaction() {
				h.sendControl(conn, "success", "Transaction aborted", nil)
			} else {
				h.sendControl(conn, "error", "No transaction to abort", nil)
			}
		case "RECONNECT":
			go func() {
				h.sendControl(conn, "processing", "Reconnecting to POS...", nil)
				if err := h.Manager.Reconnect(); err != nil {
					h.sendControl(conn, "error", err.Error(), nil)
				} else {
					h.sendControl(conn, "success", "Reconnected to POS", nil)
				}
			}()
		case "RESTART":
			h.sendControl(conn, "processing", "Server restarting...", nil)
			log.Println("RESTART command received - triggering server restart")
			go func() {
				// Give time for the message to be sent
				time.Sleep(500 * time.Millisecond)
				os.Exit(0) // Exit, expecting process manager to restart
			}()
		case "SALE", "REFUND", "SETTLEMENT", "GETPAN", "COMPLETE", "TERMINATE", "INQUIRY", "TRANSACT":
			go h.handleTransaction(conn, req)
		default:
			h.sendControl(conn, "error", "Unknown Command", nil)
		}
	}
}

func (h *Handler) sendJSON(conn *websocket.Conn, status, message, commandType string, data interface{}) {
	resp := WebResponse{
		Status:      status,
		Message:     message,
		CommandType: commandType,
		Data:        data,
	}
	if err := conn.WriteJSON(resp); err != nil {
		log.Printf("Send error: %v", err)
	}
}

func (h *Handler) sendControl(conn *websocket.Conn, status, message string, data interface{}) {
	h.sendJSON(conn, status, message, "control", data)
}

func (h *Handler) sendTransaction(conn *websocket.Conn, status, message string, data interface{}) {
	h.sendJSON(conn, status, message, "transaction", data)
}

func (h *Handler) sendStatus(conn *websocket.Conn, message string, data interface{}) {
	h.sendJSON(conn, "status_update", message, "status", data)
}

func (h *Handler) handleTransaction(conn *websocket.Conn, req WebRequest) {
	// Build Protocol Request
	fields := map[string]string{}
	for k, v := range req.Fields {
		fields[k] = v
	}
	if req.OrderNo != "" {
		if _, ok := fields["Order_No"]; !ok {
			fields["Order_No"] = req.OrderNo
		}
		if _, ok := fields["EC_Order_No"]; !ok {
			fields["EC_Order_No"] = req.OrderNo
		}
	}

	transType := req.TransType
	hostID := req.HostID
	if hostID == "" {
		hostID = "02" // default TCB credit host
	}

	switch req.Command {
	case "SALE":
		transType = "01"
		fields["Host_ID"] = hostID
		fields["Trans_Amount"] = req.Amount
	case "REFUND":
		transType = "02"
		fields["Host_ID"] = hostID
		fields["Trans_Amount"] = req.Amount
	case "SETTLEMENT":
		transType = "50"
		fields["Host_ID"] = hostID
	case "GETPAN":
		transType = "60"
		fields["Host_ID"] = hostID
		fields["Trans_Amount"] = req.Amount
	case "COMPLETE":
		transType = "62"
		fields["Host_ID"] = hostID
		fields["Trans_Amount"] = req.Amount
	case "TERMINATE":
		transType = "70"
		fields["Host_ID"] = hostID
	case "INQUIRY":
		transType = "07"
		fields["Host_ID"] = hostID
	case "TRANSACT":
		// use req.TransType + req.Fields
	default:
	}

	if transType == "" {
		h.sendTransaction(conn, "error", "Missing trans_type", nil)
		return
	}

	// Idempotency check (business layer)
	idemKey := h.idemKey(req, transType, fields)
	if idemKey != "" {
		if rec, ok := h.idemGet(idemKey); ok {
			h.replyIdem(conn, rec)
			return
		}
	}

	// Try to lock for transaction
	if !h.mu.TryLock() {
		h.sendTransaction(conn, "error", "POS is busy", nil)
		return
	}
	defer h.mu.Unlock()

	// Re-check idempotency after acquiring lock
	if idemKey != "" {
		if rec, ok := h.idemGet(idemKey); ok {
			h.replyIdem(conn, rec)
			return
		}
		h.idemSetProcessing(idemKey)
	}

	tcbReq := protocol.TCBRequest{
		TransType: transType,
		Fields:    fields,
	}

	// Execute transaction
	result, err := h.Manager.ExecuteTransaction(tcbReq)
	if err != nil {
		payload := normalizeResponse(result)
		if req.OrderNo != "" {
			payload["MerchantOrderNo"] = req.OrderNo
		}
		h.sendTransaction(conn, "error", err.Error(), payload)
		if idemKey != "" {
			h.idemSetResult(idemKey, "error", err.Error(), payload)
		}
		return
	}

	// Success
	payload := normalizeResponse(result)
	if req.OrderNo != "" {
		payload["MerchantOrderNo"] = req.OrderNo
	}
	h.sendTransaction(conn, "success", "Transaction Approved", payload)
	if idemKey != "" {
		h.idemSetResult(idemKey, "success", "Transaction Approved", payload)
	}
}

// Close stops the handler
func (h *Handler) Close() {
	close(h.stopBroadcast)
}

type idemRecord struct {
	status    string
	message   string
	data      map[string]string
	updatedAt time.Time
}

const idemTTL = 5 * time.Minute

func (h *Handler) idemKey(req WebRequest, transType string, fields map[string]string) string {
	parts := []string{
		strings.TrimSpace(req.Command),
		strings.TrimSpace(transType),
		strings.TrimSpace(req.HostID),
		strings.TrimSpace(req.Amount),
		strings.TrimSpace(req.OrderNo),
		strings.TrimSpace(fields["Invoice_No"]),
		strings.TrimSpace(fields["Reference_No"]),
		strings.TrimSpace(fields["Order_No"]),
		strings.TrimSpace(fields["EC_Order_No"]),
	}

	// Skip if no stable identifiers
	hasKey := false
	for _, p := range parts {
		if p != "" {
			hasKey = true
			break
		}
	}
	if !hasKey {
		return ""
	}

	return strings.Join(parts, "|")
}

func (h *Handler) idemGet(key string) (idemRecord, bool) {
	h.idemMu.Lock()
	defer h.idemMu.Unlock()
	h.idemCleanupLocked()

	rec, ok := h.idemCache[key]
	if !ok {
		return idemRecord{}, false
	}
	if time.Since(rec.updatedAt) > idemTTL {
		delete(h.idemCache, key)
		return idemRecord{}, false
	}
	return rec, true
}

func (h *Handler) idemSetProcessing(key string) {
	h.idemMu.Lock()
	defer h.idemMu.Unlock()
	h.idemCleanupLocked()
	h.idemCache[key] = idemRecord{
		status:    "processing",
		message:   "Duplicate transaction in progress",
		updatedAt: time.Now(),
	}
}

func (h *Handler) idemSetResult(key, status, message string, data map[string]string) {
	h.idemMu.Lock()
	defer h.idemMu.Unlock()
	h.idemCleanupLocked()
	h.idemCache[key] = idemRecord{
		status:    status,
		message:   message,
		data:      data,
		updatedAt: time.Now(),
	}
}

func (h *Handler) replyIdem(conn *websocket.Conn, rec idemRecord) {
	switch rec.status {
	case "success":
		h.sendTransaction(conn, "success", rec.message, rec.data)
	case "error":
		h.sendTransaction(conn, "error", rec.message, rec.data)
	case "processing":
		h.sendTransaction(conn, "processing", rec.message, rec.data)
	default:
		h.sendTransaction(conn, "processing", "Duplicate transaction in progress", rec.data)
	}
}

func (h *Handler) idemCleanupLocked() {
	if len(h.idemCache) == 0 {
		return
	}
	now := time.Now()
	for k, v := range h.idemCache {
		if now.Sub(v.updatedAt) > idemTTL {
			delete(h.idemCache, k)
		}
	}
}

func normalizeResponse(raw map[string]string) map[string]string {
	if raw == nil {
		return nil
	}
	data := make(map[string]string, len(raw)+8)
	for k, v := range raw {
		data[k] = v
	}

	// Common aliases for webapp compatibility
	addIfEmpty := func(key, val string) {
		if key == "" || val == "" {
			return
		}
		if _, ok := data[key]; !ok {
			data[key] = val
		}
	}

	transType := pickFirst(raw, []string{"Trans_Type", "TransType"})
	amount := pickFirst(raw, []string{"Trans_Amount", "Amount", "Sale_Amount", "Refund_Amount"})
	approval := pickFirst(raw, []string{"Approval_No"})
	orderNo := pickFirst(raw, []string{"Reference_No", "Invoice_No", "Order_No", "EC_Order_No"})
	cardNo := pickFirst(raw, []string{"Card_No", "CardAccount"})
	if cardNo == "" {
		if v := pickFirst(raw, []string{"Encrypted Card Number"}); v != "" {
			if safeMaskedCard(v) {
				cardNo = v
			}
		}
	}
	respCode := pickFirst(raw, []string{"ECR_Response_Code", "TCB_ECR_Response_Code", "FISC_ECR_Response_Code", "NP_ECR_Response_Code", "INST_ECR_Response_Code", "AE_ECR_Response_Code"})
	if respCode == "" {
		respCode = pickFirstContains(raw, "ECR_Response_Code")
	}

	addIfEmpty("TransType", transType)
	addIfEmpty("Amount", amount)
	addIfEmpty("ApprovalNo", approval)
	addIfEmpty("OrderNo", orderNo)
	addIfEmpty("CardNo", cardNo)
	addIfEmpty("RespCode", respCode)

	return data
}

func safeMaskedCard(val string) bool {
	val = strings.TrimSpace(val)
	if val == "" {
		return false
	}
	if strings.Contains(val, "*") {
		return true
	}
	// Avoid leaking encrypted blobs; allow typical PAN length only
	return len(val) <= 19
}

func pickFirst(raw map[string]string, keys []string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(raw[k]); v != "" {
			return v
		}
	}
	return ""
}

func pickFirstContains(raw map[string]string, needle string) string {
	for k, v := range raw {
		if strings.Contains(k, needle) && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
