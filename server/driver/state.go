package driver

import (
	"sync"
	"time"
)

// TransactionState represents the current state of the POS transaction
type TransactionState int

const (
	StateIdle TransactionState = iota
	StateSending
	StateWaitACK
	StateWaitResponse
	StateParsing
	StateSuccess
	StateError
	StateTimeout
)

// String returns the string representation of the state
func (s TransactionState) String() string {
	switch s {
	case StateIdle:
		return "IDLE"
	case StateSending:
		return "SENDING"
	case StateWaitACK:
		return "WAIT_ACK"
	case StateWaitResponse:
		return "WAIT_RESPONSE"
	case StateParsing:
		return "PARSING"
	case StateSuccess:
		return "SUCCESS"
	case StateError:
		return "ERROR"
	case StateTimeout:
		return "TIMEOUT"
	default:
		return "UNKNOWN"
	}
}

// StateTimeout defines maximum duration for each state
var StateTimeouts = map[TransactionState]time.Duration{
	StateSending:      2 * time.Second,
	StateWaitACK:      5 * time.Second,
	StateWaitResponse: 65 * time.Second,
	StateParsing:      2 * time.Second,
}

// StatusInfo contains detailed status information for broadcasting
type StatusInfo struct {
	State       string    `json:"state"`
	Message     string    `json:"message"`
	StartedAt   time.Time `json:"started_at,omitempty"`
	ElapsedMs   int64     `json:"elapsed_ms"`
	TimeoutMs   int64     `json:"timeout_ms,omitempty"`
	LastError   string    `json:"last_error,omitempty"`
	TransType   string    `json:"trans_type,omitempty"`
	Amount      string    `json:"amount,omitempty"`
	IsConnected bool      `json:"is_connected"`
}

// StateChangeCallback is called when state changes
type StateChangeCallback func(info StatusInfo)

// StateMachine manages transaction state with thread-safety
type StateMachine struct {
	mu sync.RWMutex

	currentState TransactionState
	stateStarted time.Time
	lastError    string
	transType    string
	amount       string
	isConnected  bool

	cancelChan    chan struct{}
	onStateChange StateChangeCallback
}

// NewStateMachine creates a new state machine
func NewStateMachine() *StateMachine {
	return &StateMachine{
		currentState: StateIdle,
		isConnected:  false,
		cancelChan:   make(chan struct{}),
	}
}

// SetCallback sets the state change callback
func (sm *StateMachine) SetCallback(cb StateChangeCallback) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.onStateChange = cb
}

// SetConnected sets the connection status
func (sm *StateMachine) SetConnected(connected bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.isConnected = connected
	if sm.onStateChange != nil {
		sm.onStateChange(sm.getStatusInfoLocked())
	}
}

// GetState returns the current state
func (sm *StateMachine) GetState() TransactionState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.currentState
}

// GetStatusInfo returns the current status information
func (sm *StateMachine) GetStatusInfo() StatusInfo {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.getStatusInfoLocked()
}

func (sm *StateMachine) getStatusInfoLocked() StatusInfo {
	info := StatusInfo{
		State:       sm.currentState.String(),
		IsConnected: sm.isConnected,
		LastError:   sm.lastError,
		TransType:   sm.transType,
		Amount:      sm.amount,
	}

	if sm.currentState != StateIdle {
		info.StartedAt = sm.stateStarted
		info.ElapsedMs = time.Since(sm.stateStarted).Milliseconds()
		if timeout, ok := StateTimeouts[sm.currentState]; ok {
			info.TimeoutMs = timeout.Milliseconds()
		}
	}

	// Generate message based on state
	switch sm.currentState {
	case StateIdle:
		info.Message = "Ready for transaction"
	case StateSending:
		info.Message = "Sending request to POS..."
	case StateWaitACK:
		info.Message = "Waiting for POS acknowledgement..."
	case StateWaitResponse:
		info.Message = "Waiting for card operation..."
	case StateParsing:
		info.Message = "Processing response..."
	case StateSuccess:
		info.Message = "Transaction approved"
	case StateError:
		info.Message = "Transaction failed: " + sm.lastError
	case StateTimeout:
		info.Message = "Transaction timed out"
	}

	return info
}

// TransitionTo changes to a new state
func (sm *StateMachine) TransitionTo(newState TransactionState) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.currentState = newState
	sm.stateStarted = time.Now()

	// Clear error on non-error states
	if newState != StateError && newState != StateTimeout {
		sm.lastError = ""
	}

	// Notify callback
	if sm.onStateChange != nil {
		sm.onStateChange(sm.getStatusInfoLocked())
	}
}

// TransitionToError transitions to error state with a message
func (sm *StateMachine) TransitionToError(err string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.currentState = StateError
	sm.stateStarted = time.Now()
	sm.lastError = err

	if sm.onStateChange != nil {
		sm.onStateChange(sm.getStatusInfoLocked())
	}
}

// TransitionToTimeout transitions to timeout state
func (sm *StateMachine) TransitionToTimeout() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.currentState = StateTimeout
	sm.stateStarted = time.Now()
	sm.lastError = "operation timed out"

	if sm.onStateChange != nil {
		sm.onStateChange(sm.getStatusInfoLocked())
	}
}

// StartTransaction initializes a new transaction
func (sm *StateMachine) StartTransaction(transType, amount string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.currentState != StateIdle {
		return ErrTransactionInProgress
	}

	sm.transType = transType
	sm.amount = amount
	sm.lastError = ""
	sm.cancelChan = make(chan struct{})

	return nil
}

// Reset returns the state machine to idle
func (sm *StateMachine) Reset() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.currentState = StateIdle
	sm.transType = ""
	sm.amount = ""
	sm.stateStarted = time.Time{}

	if sm.onStateChange != nil {
		sm.onStateChange(sm.getStatusInfoLocked())
	}
}

// Abort attempts to cancel the current transaction
func (sm *StateMachine) Abort() bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.currentState == StateIdle {
		return false
	}

	// Signal cancellation
	select {
	case <-sm.cancelChan:
		// Already closed
	default:
		close(sm.cancelChan)
	}

	sm.currentState = StateError
	sm.lastError = "aborted by user"
	sm.stateStarted = time.Now()

	if sm.onStateChange != nil {
		sm.onStateChange(sm.getStatusInfoLocked())
	}

	return true
}

// GetCancelChannel returns the cancellation channel
func (sm *StateMachine) GetCancelChannel() <-chan struct{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.cancelChan
}

// IsTimedOut checks if the current state has exceeded its timeout
func (sm *StateMachine) IsTimedOut() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	timeout, ok := StateTimeouts[sm.currentState]
	if !ok {
		return false
	}

	return time.Since(sm.stateStarted) > timeout
}

// Error definitions
var ErrTransactionInProgress = &TransactionError{Message: "transaction already in progress"}

type TransactionError struct {
	Message string
}

func (e *TransactionError) Error() string {
	return e.Message
}
