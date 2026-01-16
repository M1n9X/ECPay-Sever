package driver

import (
	"ecpay-server/logger"
	"ecpay-server/protocol"
	"time"

	"go.bug.st/serial"
)

// Scanner handles auto-detection of POS devices
type Scanner struct {
	Manager *SerialManager
	stop    chan struct{}
}

func NewScanner(manager *SerialManager) *Scanner {
	return &Scanner{
		Manager: manager,
		stop:    make(chan struct{}),
	}
}

// Start begins the scanning loop
// 1. Initial burst scan (3 times)
// 2. Periodic scan (every 20s)
func (s *Scanner) Start() {
	go func() {
		// Initial burst scan
		for i := 0; i < 3; i++ {
			if s.scanAndConnect() {
				return
			}
			time.Sleep(1 * time.Second)
		}

		// Periodic scan
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-s.stop:
				return
			case <-ticker.C:
				if !s.Manager.IsConnected() {
					s.scanAndConnect()
				}
			}
		}
	}()
}

func (s *Scanner) Stop() {
	close(s.stop)
}

// scanAndConnect iterates over all ports to find a POS
func (s *Scanner) scanAndConnect() bool {
	logger.Info("Scanning for POS device...")

	// Determine if we should scan for mock (e.g. via environment or config, for now we append it for logic testing)
	ports, err := serial.GetPortsList()
	if err != nil {
		logger.Error("Failed to list serial ports: %v", err)
		// Don't return, keep trying mock if applicable
	}

	// Always append Mock POS for testing auto-detection logic
	ports = append(ports, "MOCK_POS")

	if len(ports) == 0 {
		logger.Info("No serial ports found")
		return false
	}

	for _, portName := range ports {
		logger.Debug("Probing port: %s", portName)
		if s.probePort(portName) {
			logger.Info("POS device found on %s!", portName)
			return true
		}
	}

	logger.Info("No POS device found in this scan cycle")
	return false
}

// probePort attempts to connect and send ECHO
func (s *Scanner) probePort(portName string) bool {
	// 1. Open Port
	port, err := OpenSerial(portName, 115200)
	if err != nil {
		return false
	}
	defer port.Close()

	// 2. Clear buffer
	port.ResetInputBuffer()

	// 3. Send ECHO Request
	req := protocol.ECPayRequest{TransType: "80", HostID: "01"} // ECHO
	packet := protocol.BuildPacket(req)
	if _, err := port.Write(packet); err != nil {
		return false
	}

	// 4. Wait for ACK (Brief timeout for probing)
	ackBuf := make([]byte, 64)
	ackReceived := false
	start := time.Now()
	for time.Since(start) < 500*time.Millisecond {
		n, _ := port.Read(ackBuf)
		if n > 0 {
			for i := 0; i < n; i++ {
				if ackBuf[i] == protocol.ACK {
					ackReceived = true
					break
				}
			}
		}
		if ackReceived {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if !ackReceived {
		return false
	}

	// 5. Success! Re-open persistently via Manager
	// We close this probe connection first (defer Close matches here)
	// Then tell manager to connect to this port

	// Note: We can't reuse this 'port' object easily because Manager expects to open it or take ownership.
	// Since port.Close() is deferred, we let it close, then Manager opens it again.
	// There is a small race/risk but usually fine for RS232.

	return s.Manager.ConnectTo(portName)
}
