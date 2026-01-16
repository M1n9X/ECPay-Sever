package driver

import (
	"ecpay-server/logger"
	"ecpay-server/protocol"
	"runtime"
	"strings"
	"time"

	"go.bug.st/serial"
)

// Scanner handles auto-detection of POS devices on serial ports
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
// 1. Initial burst scan (3 attempts with 1s interval)
// 2. Periodic scan (every 20s if not connected)
func (s *Scanner) Start() {
	go func() {
		logger.Info("Starting POS device scanner...")

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
				logger.Info("Scanner stopped")
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

// scanAndConnect iterates over all serial ports to find a POS device
func (s *Scanner) scanAndConnect() bool {
	logger.Info("Scanning for POS device...")

	ports, err := serial.GetPortsList()
	if err != nil {
		logger.Error("Failed to list serial ports: %v", err)
		return false
	}

	if len(ports) == 0 {
		logger.Info("No serial ports found")
		return false
	}

	// Filter ports based on OS conventions
	filteredPorts := filterPorts(ports)
	logger.Debug("Found %d candidate ports: %v", len(filteredPorts), filteredPorts)

	for _, portName := range filteredPorts {
		logger.Debug("Probing port: %s", portName)
		if s.probePort(portName) {
			logger.Info("POS device found on %s", portName)
			return true
		}
	}

	logger.Info("No POS device found in this scan cycle")
	return false
}

// filterPorts filters serial ports based on OS-specific naming conventions
func filterPorts(ports []string) []string {
	var filtered []string

	for _, port := range ports {
		// Windows: COM ports
		if runtime.GOOS == "windows" {
			if strings.HasPrefix(strings.ToUpper(port), "COM") {
				filtered = append(filtered, port)
			}
			continue
		}

		// macOS/Linux: filter out Bluetooth and other non-serial devices
		lower := strings.ToLower(port)

		// Skip Bluetooth ports
		if strings.Contains(lower, "bluetooth") {
			continue
		}

		// Include common USB-Serial adapters
		if strings.Contains(lower, "ttyusb") || // Linux USB-Serial
			strings.Contains(lower, "ttyacm") || // Linux ACM devices
			strings.Contains(lower, "usbserial") || // macOS USB-Serial
			strings.Contains(lower, "cu.") { // macOS serial ports
			filtered = append(filtered, port)
		}
	}

	return filtered
}

// probePort attempts to connect and send ECHO command to verify POS device
func (s *Scanner) probePort(portName string) bool {
	// 1. Open Port
	port, err := OpenSerial(portName, 115200)
	if err != nil {
		logger.Debug("Failed to open %s: %v", portName, err)
		return false
	}
	defer port.Close()

	// 2. Clear input buffer
	port.ResetInputBuffer()

	// 3. Send ECHO Request (TransType 80)
	req := protocol.ECPayRequest{
		TransType: "80", // ECHO
		HostID:    "01", // Credit Card
	}
	packet := protocol.BuildPacket(req)

	if _, err := port.Write(packet); err != nil {
		logger.Debug("Failed to write to %s: %v", portName, err)
		return false
	}

	// 4. Wait for ACK (500ms timeout for probing)
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
				if ackBuf[i] == protocol.NAK {
					// NAK means device responded but rejected - still a valid POS
					logger.Debug("Received NAK from %s (device present but rejected ECHO)", portName)
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
		logger.Debug("No response from %s", portName)
		return false
	}

	// 5. Device found - close probe connection and let Manager open it
	// Note: There's a small window between close and reopen, but RS232 handles this fine
	port.Close()

	return s.Manager.ConnectTo(portName)
}
