package driver

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"tcb-server/logger"
	"tcb-server/protocol"
)

const (
	AckTimeout       = 1 * time.Second
	ResponseTimeout  = 60 * time.Second
	MaxRetries       = 3
	ResponseMaxRetry = 3
)

// SerialManager manages the serial port connection and transaction execution
// TCB Flow: Send -> Wait ACK/NAK (or timeout as ACK) -> Wait Response -> ACKx2 -> Parse
// ACK/NAK are always sent twice.
// ACK timeout is treated as ACK (EDC Ack Lose).
// Retries only on NAK with 1s delay.
// Response errors (LRC/length) -> NAKx2 and retry up to 3 times.
//
// NOTE: Single transaction at a time. Use StateMachine to enforce.
//
// Design based on docs/TCB/TCB_FSM.md

type SerialManager struct {
	Port    Port
	State   *StateMachine
	Scanner *Scanner
	Baud    int
	mu      sync.Mutex // Protects Port access during reconnection
}

// NewSerialManager creates a new manager with optional initial port
// If initialPort is nil and autoDetect is true, scanner will be started
func NewSerialManager(initialPort Port, baud int, autoDetect bool) *SerialManager {
	sm := &SerialManager{
		Port:  initialPort,
		State: NewStateMachine(),
		Baud:  baud,
	}

	if initialPort != nil {
		sm.State.SetConnected(true)
	} else if autoDetect {
		sm.State.SetConnected(false)
		sm.Scanner = NewScanner(sm)
		sm.Scanner.Start()
	} else {
		sm.State.SetConnected(false)
	}

	return sm
}

// ConnectTo connects to a specific serial port
func (sm *SerialManager) ConnectTo(portName string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.Port != nil {
		sm.Port.Close()
		sm.Port = nil
	}

	logger.Info("Connecting to %s...", portName)
	port, err := OpenSerial(portName, sm.Baud)
	if err != nil {
		logger.Error("Failed to connect to %s: %v", portName, err)
		sm.State.SetConnected(false)
		return false
	}

	sm.Port = port
	sm.State.SetConnected(true)
	logger.Info("Connected to %s", portName)
	return true
}

// Disconnect closes the current connection
func (sm *SerialManager) Disconnect() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.Port != nil {
		sm.Port.Close()
		sm.Port = nil
	}
	sm.State.SetConnected(false)
}

// IsConnected checks if a device is currently connected
func (sm *SerialManager) IsConnected() bool {
	return sm.State.IsConnected()
}

// ForceRescan triggers a manual scan for POS devices
func (sm *SerialManager) ForceRescan() {
	if sm.Scanner != nil {
		go sm.Scanner.scanAndConnect()
	}
}

// Reconnect attempts to reconnect to the POS device
func (sm *SerialManager) Reconnect() error {
	logger.Info("Reconnect requested...")

	sm.Disconnect()

	if sm.Scanner != nil {
		sm.ForceRescan()
		return nil
	}

	return errors.New("no scanner available for reconnection")
}

// SetStateCallback sets the callback for state changes
func (sm *SerialManager) SetStateCallback(cb StateChangeCallback) {
	sm.State.SetCallback(cb)
}

// GetStatus returns the current status
func (sm *SerialManager) GetStatus() StatusInfo {
	return sm.State.GetStatusInfo()
}

// AbortTransaction attempts to cancel the current transaction
func (sm *SerialManager) AbortTransaction() bool {
	return sm.State.Abort()
}

// ExecuteTransaction executes a complete TCB transaction
func (sm *SerialManager) ExecuteTransaction(req protocol.TCBRequest) (map[string]string, error) {
	logger.Info("Starting transaction: Type=%s", req.TransType)

	if !sm.IsConnected() || sm.Port == nil {
		return nil, errors.New("POS device not connected")
	}

	if err := sm.State.StartTransaction(req.TransType, ""); err != nil {
		logger.Error("Cannot start transaction: %v", err)
		return nil, err
	}

	defer func() {
		if sm.State.GetState() == StateError {
			time.Sleep(2 * time.Second)
		}
		sm.State.Reset()
		logger.Debug("Transaction state reset to IDLE")
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 70*time.Second)
	defer cancel()
	cancelChan := sm.State.GetCancelChannel()

	// Build packet
	sm.State.TransitionTo(StateSending)
	packet, err := protocol.BuildPacket(req)
	if err != nil {
		sm.State.TransitionToError(err.Error())
		return nil, err
	}

	// Clear input buffer
	if err := sm.Port.ResetInputBuffer(); err != nil {
		logger.Warn("Failed to reset input buffer: %v", err)
	}

	// Send with retry on NAK
	var prebuf []byte
	if err := sm.sendWithRetry(ctx, cancelChan, packet, &prebuf); err != nil {
		sm.State.TransitionToError(err.Error())
		return nil, err
	}

	// Wait for Response
	sm.State.TransitionTo(StateWaitResponse)
	resp, err := sm.waitForResponse(ctx, cancelChan, prebuf)
	if err != nil {
		if errors.Is(err, context.Canceled) || err.Error() == "aborted" {
			return nil, errors.New("transaction aborted")
		}
		if err.Error() == "timeout" {
			sm.State.TransitionToTimeout()
			return nil, errors.New("transaction timeout")
		}
		sm.State.TransitionToError(err.Error())
		return nil, err
	}

	// Parse response
	sm.State.TransitionTo(StateParsing)
	result, err := protocol.ParseResponse(resp)
	if err != nil {
		sm.State.TransitionToError(err.Error())
		return nil, err
	}

	// Evaluate response code (if present)
	if code, ok := result["ECR_Response_Code"]; ok && code != "" && code != "0000" {
		errMsg := fmt.Sprintf("transaction declined: %s", code)
		sm.State.TransitionToError(errMsg)
		return result, errors.New(errMsg)
	}

	sm.State.TransitionTo(StateSuccess)
	return result, nil
}

func (sm *SerialManager) sendWithRetry(ctx context.Context, cancelChan <-chan struct{}, packet []byte, prebuf *[]byte) error {
	sm.State.TransitionTo(StateWaitACK)

	for attempt := 0; attempt <= MaxRetries; attempt++ {
		if _, err := sm.Port.Write(packet); err != nil {
			sm.handleWriteError(err)
			return fmt.Errorf("write error: %v", err)
		}
		logger.Debug("Packet sent (%d bytes)", len(packet))

		res, err := sm.waitAckOrStx(ctx, cancelChan)
		if err != nil {
			if errors.Is(err, context.Canceled) || err.Error() == "aborted" {
				return errors.New("transaction aborted")
			}
			return err
		}

		switch res.kind {
		case ackNakAck, ackNakTimeout:
			// ACK received or timeout => assume ACK
			return nil
		case ackNakStx:
			*prebuf = res.prebuf
			return nil
		case ackNakNak:
			// NAK => retry after 1s
			if attempt == MaxRetries {
				return errors.New("max retries exceeded on NAK")
			}
			time.Sleep(1 * time.Second)
			continue
		default:
			return errors.New("unknown ACK state")
		}
	}

	return errors.New("max retries exceeded on NAK")
}

type ackResult struct {
	kind   string
	prebuf []byte
}

const (
	ackNakAck     = "ACK"
	ackNakNak     = "NAK"
	ackNakTimeout = "TIMEOUT"
	ackNakStx     = "STX"
)

func (sm *SerialManager) waitAckOrStx(ctx context.Context, cancelChan <-chan struct{}) (ackResult, error) {
	deadline := time.Now().Add(AckTimeout)
	buf := make([]byte, 256)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ackResult{}, ctx.Err()
		case <-cancelChan:
			return ackResult{}, errors.New("aborted")
		default:
		}

		n, err := sm.Port.Read(buf)
		if n > 0 {
			for i := 0; i < n; i++ {
				switch buf[i] {
				case protocol.STX:
					return ackResult{kind: ackNakStx, prebuf: append([]byte{}, buf[i:n]...)}, nil
				case protocol.ACK:
					return ackResult{kind: ackNakAck}, nil
				case protocol.NAK:
					return ackResult{kind: ackNakNak}, nil
				}
			}
		}
		if err != nil && !isTimeoutError(err) {
			logger.Warn("Read error during ACK wait: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Timeout -> assume ACK (EDC Ack Lose)
	return ackResult{kind: ackNakTimeout}, nil
}

func (sm *SerialManager) waitForResponse(ctx context.Context, cancelChan <-chan struct{}, prebuf []byte) ([]byte, error) {
	buf := make([]byte, 1024)
	respBuf := new(bytes.Buffer)
	if len(prebuf) > 0 {
		respBuf.Write(prebuf)
	}

	attempts := 0
	start := time.Now()

	for {
		if time.Since(start) > ResponseTimeout {
			return nil, errors.New("timeout")
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-cancelChan:
			return nil, errors.New("aborted")
		default:
		}

		n, err := sm.Port.Read(buf)
		if n > 0 {
			respBuf.Write(buf[:n])
			packet, ok := tryExtractPacket(respBuf.Bytes())
			if ok {
				if !protocol.ValidatePacket(packet) {
					// LRC/length error => NAKx2 and retry
					sm.writeAck(protocol.NAK)
					attempts++
					if attempts >= ResponseMaxRetry {
						return nil, errors.New("response checksum error")
					}
					respBuf.Reset()
					continue
				}
				// ACK twice
				sm.writeAck(protocol.ACK)
				return packet, nil
			}
		}
		if err != nil && !isTimeoutError(err) {
			logger.Warn("Read error during response wait: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func tryExtractPacket(data []byte) ([]byte, bool) {
	idx := bytes.IndexByte(data, protocol.STX)
	if idx < 0 {
		return nil, false
	}
	if len(data) < idx+protocol.FrameLen {
		return nil, false
	}
	pkt := data[idx : idx+protocol.FrameLen]
	return pkt, true
}

func (sm *SerialManager) writeAck(b byte) {
	_, _ = sm.Port.Write([]byte{b, b})
}

// handleWriteError handles write errors and marks connection as lost
func (sm *SerialManager) handleWriteError(err error) {
	logger.Error("Write error (connection may be lost): %v", err)
	sm.State.TransitionToError(fmt.Sprintf("write error: %v", err))
	sm.State.SetConnected(false)

	if sm.Scanner != nil {
		go sm.Scanner.scanAndConnect()
	}
}

type timeoutError interface {
	Timeout() bool
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if te, ok := err.(timeoutError); ok && te.Timeout() {
		return true
	}
	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "timed out")
}
