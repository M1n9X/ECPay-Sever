package driver

import (
	"bytes"
	"context"
	"ecpay-server/logger"
	"ecpay-server/protocol"
	"errors"
	"fmt"
	"sync"
	"time"
)

// SerialManager manages the serial port connection and transaction execution
type SerialManager struct {
	Port    Port
	State   *StateMachine
	Scanner *Scanner
	mu      sync.Mutex // Protects Port access during reconnection
}

// NewSerialManager creates a new manager with optional initial port
// If initialPort is nil, auto-detection scanner will be started
func NewSerialManager(initialPort Port) *SerialManager {
	sm := &SerialManager{
		Port:  initialPort,
		State: NewStateMachine(),
	}

	if initialPort != nil {
		sm.State.SetConnected(true)
	} else {
		sm.State.SetConnected(false)
		// Start scanner for auto-detection
		sm.Scanner = NewScanner(sm)
		sm.Scanner.Start()
	}

	return sm
}

// ConnectTo connects to a specific serial port
func (sm *SerialManager) ConnectTo(portName string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Close existing connection if any
	if sm.Port != nil {
		sm.Port.Close()
		sm.Port = nil
	}

	logger.Info("Connecting to %s...", portName)
	port, err := OpenSerial(portName, 115200)
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

// ExecuteTransaction executes a complete ECPay transaction
// Flow: Send -> Wait ACK -> Wait Response -> Send ACK -> Parse
func (sm *SerialManager) ExecuteTransaction(req protocol.ECPayRequest) (map[string]string, error) {
	logger.Info("Starting transaction: Type=%s Amount=%s OrderNo=%s", req.TransType, req.Amount, req.OrderNo)

	// Check connection
	if !sm.IsConnected() || sm.Port == nil {
		return nil, errors.New("POS device not connected")
	}

	// Check if we can start a transaction
	if err := sm.State.StartTransaction(req.TransType, req.Amount); err != nil {
		logger.Error("Cannot start transaction: %v", err)
		return nil, err
	}

	// Ensure we always reset to IDLE when done
	defer func() {
		// Give UI time to see error state before resetting
		if sm.State.GetState() == StateError {
			time.Sleep(2 * time.Second)
		}
		sm.State.Reset()
		logger.Debug("Transaction state reset to IDLE")
	}()

	// Create context with overall timeout (70s covers all phases)
	ctx, cancel := context.WithTimeout(context.Background(), 70*time.Second)
	defer cancel()

	// Get cancel channel for user abort
	cancelChan := sm.State.GetCancelChannel()

	// 1. Build packet
	sm.State.TransitionTo(StateSending)
	logger.Debug("Building packet...")
	packet := protocol.BuildPacket(req)

	// 2. Clear input buffer
	if err := sm.Port.ResetInputBuffer(); err != nil {
		logger.Warn("Failed to reset input buffer: %v", err)
	}

	// 3. Send packet
	_, err := sm.Port.Write(packet)
	if err != nil {
		sm.handleWriteError(err)
		return nil, fmt.Errorf("write error: %v", err)
	}
	logger.Debug("Packet sent (%d bytes)", len(packet))

	// 4. Wait for ACK (5s timeout)
	sm.State.TransitionTo(StateWaitACK)
	ackResult, err := sm.waitForACK(ctx, cancelChan)
	if err != nil {
		if errors.Is(err, context.Canceled) || err.Error() == "aborted" {
			return nil, errors.New("transaction aborted")
		}
		sm.State.TransitionToError(err.Error())
		return nil, err
	}
	if !ackResult {
		sm.State.TransitionToError("received NAK from POS")
		return nil, errors.New("received NAK from POS")
	}
	logger.Debug("ACK received")

	// 5. Wait for Response (65s timeout - user interaction time)
	sm.State.TransitionTo(StateWaitResponse)
	logger.Info("Waiting for POS response (card operation)...")

	responsePacket, err := sm.waitForResponse(ctx, cancelChan)
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

	// 6. Parse response
	sm.State.TransitionTo(StateParsing)

	// Validate packet LRC
	if !protocol.ValidatePacket(responsePacket) {
		sm.State.TransitionToError("invalid packet checksum")
		return nil, errors.New("invalid packet checksum")
	}

	// Send ACK back to POS
	if _, err := sm.Port.Write([]byte{protocol.ACK}); err != nil {
		logger.Warn("Failed to send ACK: %v", err)
	}

	// Parse response fields
	result := protocol.ParseResponse(responsePacket)
	logger.Info("Response parsed: RespCode=%s ApprovalNo=%s", result["RespCode"], result["ApprovalNo"])

	// Check response code
	if respCode, ok := result["RespCode"]; ok && respCode != "0000" {
		errMsg := fmt.Sprintf("transaction declined: %s", respCode)
		sm.State.TransitionToError(errMsg)
		return result, errors.New(errMsg)
	}

	// Success
	sm.State.TransitionTo(StateSuccess)
	return result, nil
}

// handleWriteError handles write errors and marks connection as lost
func (sm *SerialManager) handleWriteError(err error) {
	logger.Error("Write error (connection may be lost): %v", err)
	sm.State.TransitionToError(fmt.Sprintf("write error: %v", err))
	sm.State.SetConnected(false)

	// Trigger rescan to find device again
	if sm.Scanner != nil {
		go sm.Scanner.scanAndConnect()
	}
}

// waitForACK waits for ACK/NAK with timeout and cancellation support
func (sm *SerialManager) waitForACK(ctx context.Context, cancelChan <-chan struct{}) (bool, error) {
	timeout := time.After(5 * time.Second)
	buf := make([]byte, 64)

	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-cancelChan:
			return false, errors.New("aborted")
		case <-timeout:
			return false, errors.New("timeout waiting for ACK")
		default:
			n, err := sm.Port.Read(buf)
			if n > 0 {
				for i := 0; i < n; i++ {
					if buf[i] == protocol.ACK {
						return true, nil
					}
					if buf[i] == protocol.NAK {
						return false, nil
					}
				}
			}
			// Ignore timeout errors from Read (expected due to SetReadTimeout)
			if err != nil && !isTimeoutError(err) {
				logger.Warn("Read error during ACK wait: %v", err)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
}

// waitForResponse waits for complete response packet
func (sm *SerialManager) waitForResponse(ctx context.Context, cancelChan <-chan struct{}) ([]byte, error) {
	timeout := time.After(65 * time.Second)
	buf := make([]byte, 1024)
	respBuffer := new(bytes.Buffer)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-cancelChan:
			return nil, errors.New("aborted")
		case <-timeout:
			return nil, errors.New("timeout")
		default:
			n, err := sm.Port.Read(buf)
			if n > 0 {
				respBuffer.Write(buf[:n])

				// Check for complete packet (STX + 600 DATA + ETX + LRC = 603 bytes)
				data := respBuffer.Bytes()
				idxStx := bytes.IndexByte(data, protocol.STX)
				idxEtx := bytes.LastIndexByte(data, protocol.ETX)

				if idxStx >= 0 && idxEtx > idxStx && len(data) >= idxEtx+2 {
					// Extract complete packet (STX to LRC inclusive)
					packetData := data[idxStx : idxEtx+2]
					if len(packetData) == 603 {
						return packetData, nil
					}
				}
			}
			// Ignore timeout errors from Read
			if err != nil && !isTimeoutError(err) {
				logger.Warn("Read error during response wait: %v", err)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// Reconnect attempts to reconnect to the POS device
func (sm *SerialManager) Reconnect() error {
	logger.Info("Reconnect requested...")

	// Mark as disconnected
	sm.Disconnect()

	// Trigger scanner to find device
	if sm.Scanner != nil {
		sm.ForceRescan()
		return nil
	}

	return errors.New("no scanner available for reconnection")
}

// isTimeoutError checks if an error is a timeout (expected from SetReadTimeout)
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	// go.bug.st/serial returns timeout as a specific error
	return err.Error() == "timeout" || err.Error() == "EOF"
}
