package api

import (
	"ecpay-server/driver"
	"ecpay-server/protocol"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type WebRequest struct {
	Command string `json:"command"` // "SALE", "REFUND", "STATUS", "ABORT", "RECONNECT"
	Amount  string `json:"amount"`
	OrderNo string `json:"order_no"`
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
}

func NewHandler(manager *driver.SerialManager) *Handler {
	h := &Handler{
		Manager:       manager,
		clients:       make(map[*websocket.Conn]bool),
		stopBroadcast: make(chan struct{}),
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
		case "SALE", "REFUND", "SETTLEMENT", "ECHO":
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
	// Try to lock for transaction
	if !h.mu.TryLock() {
		h.sendTransaction(conn, "error", "POS is busy", nil)
		return
	}
	defer h.mu.Unlock()

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
		ecpayReq.Amount = "0"
	case "ECHO":
		ecpayReq.TransType = "80"
		ecpayReq.HostID = "01"
	}

	// Execute transaction
	result, err := h.Manager.ExecuteTransaction(ecpayReq)
	if err != nil {
		h.sendTransaction(conn, "error", err.Error(), result)
		return
	}

	// Success
	h.sendTransaction(conn, "success", "Transaction Approved", result)
}

// Close stops the handler
func (h *Handler) Close() {
	close(h.stopBroadcast)
}
