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

type SerialManager struct {
	Port    Port
	State   *StateMachine
	Scanner *Scanner
	mu      sync.Mutex // Protects Port access during reconnection
}

func NewSerialManager(initialPort Port) *SerialManager {
	sm := &SerialManager{
		Port:  initialPort,
		State: NewStateMachine(),
	}

	if initialPort != nil {
		sm.State.SetConnected(true)
	} else {
		sm.State.SetConnected(false)
		// Start scanner if no initial port
		sm.Scanner = NewScanner(sm)
		sm.Scanner.Start()
	}

	return sm
}

// ConnectTo connects to a specific serial port
func (sm *SerialManager) ConnectTo(portName string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.Port != nil {
		sm.Port.Close()
	}

	logger.Info("Connecting to %s...", portName)
	port, err := OpenSerial(portName, 115200)
	if err != nil {
		logger.Error("Failed to connect to %s: %v", portName, err)
		return false
	}

	sm.Port = port
	sm.State.SetConnected(true)
	return true
}

// IsConnected checks if a device is currently connected
func (sm *SerialManager) IsConnected() bool {
	return sm.State.IsConnected() // State machine tracks this
}

// ForceRescan triggers a manual scan
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

// ExecuteTransaction executes a complete ECPay transaction with state machine
// Flow: Send -> Wait ACK -> Wait Response -> Send ACK
func (sm *SerialManager) ExecuteTransaction(req protocol.ECPayRequest) (map[string]string, error) {
	logger.Info("Starting transaction: Type=%s Amount=%s OrderNo=%s", req.TransType, req.Amount, req.OrderNo)

	// Check if we can start a transaction
	if err := sm.State.StartTransaction(req.TransType, req.Amount); err != nil {
		logger.Error("Cannot start transaction: %v", err)
		return nil, err
	}

	// Ensure we always reset to IDLE when done
	defer func() {
		// If we are in an error state (like aborted), give the UI a moment to see it
		// before resetting to IDLE
		if sm.State.GetState() == StateError {
			time.Sleep(2 * time.Second)
		}

		sm.State.Reset()
		logger.Debug("Transaction state reset to IDLE")
	}()

	// Create context with overall timeout
	ctx, cancel := context.WithTimeout(context.Background(), 70*time.Second)
	defer cancel()

	// Get cancel channel for user abort
	cancelChan := sm.State.GetCancelChannel()

	// 1. Build packet
	sm.State.TransitionTo(StateSending)
	logger.Debug("Building packet...")
	packet := protocol.BuildPacket(req)

	// 2. Clear input buffer
	sm.Port.ResetInputBuffer()

	// 3. Send packet
	_, err := sm.Port.Write(packet)
	if err != nil {
		sm.State.TransitionToError(fmt.Sprintf("write error: %v", err))
		return nil, fmt.Errorf("write error: %v", err)
	}

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

	// 5. Wait for Response (65s timeout)
	sm.State.TransitionTo(StateWaitResponse)
	fmt.Println("ACK received. Waiting for POS Response...")

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

	// Validate packet
	if !protocol.ValidatePacket(responsePacket) {
		sm.State.TransitionToError("invalid packet checksum")
		return nil, errors.New("invalid packet checksum")
	}

	// Send ACK back
	sm.Port.Write([]byte{protocol.ACK})

	// Parse response
	result := protocol.ParseResponse(responsePacket)

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
			if err != nil && err.Error() != "EOF" {
				// Ignore EOF for mock
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

				// Check for complete packet
				data := respBuffer.Bytes()
				idxStx := bytes.IndexByte(data, protocol.STX)
				idxEtx := bytes.LastIndexByte(data, protocol.ETX)

				if idxStx >= 0 && idxEtx > idxStx && len(data) > idxEtx+1 {
					// Extract complete packet
					packetData := data[idxStx : idxEtx+2] // +2 to include LRC
					return packetData, nil
				}
			}
			if err != nil {
				// Handle error
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// Reconnect attempts to reconnect to the POS
func (sm *SerialManager) Reconnect() error {
	sm.State.SetConnected(false)

	// If using scanner (Real Mode), force a rescan
	if sm.Scanner != nil {
		logger.Info("Forcing scanner rescan...")
		sm.ForceRescan()
		return nil
	}

	// Mock Mode Simulation
	time.Sleep(500 * time.Millisecond)
	sm.State.SetConnected(true)
	return nil
}
