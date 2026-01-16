package driver

import (
	"ecpay-server/logger"
	"ecpay-server/protocol"
	"fmt"
	"runtime"
	"strings"
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

// scanAndConnect finds and connects to a POS device
func (s *Scanner) scanAndConnect() bool {
	logger.Info("Scanning for POS device...")

	ports := s.discoverPorts()

	if len(ports) == 0 {
		logger.Info("No candidate ports found")
		return false
	}

	logger.Debug("Found %d candidate ports: %v", len(ports), ports)

	for _, portName := range ports {
		logger.Debug("Probing port: %s", portName)
		if s.probePort(portName) {
			logger.Info("POS device found on %s", portName)
			return true
		}
	}

	logger.Info("No POS device found in this scan cycle")
	return false
}

// discoverPorts finds all candidate ports (serial + TCP mock)
func (s *Scanner) discoverPorts() []string {
	var ports []string

	// 1. Hardware serial ports
	hwPorts, err := serial.GetPortsList()
	if err != nil {
		logger.Error("Failed to list serial ports: %v", err)
	} else {
		ports = append(ports, hwPorts...)
	}

	// 2. Mock POS TCP endpoint (for development)
	// Always try localhost:9999 as a potential mock POS
	ports = append(ports, "tcp://localhost:9999")

	// 3. Filter and deduplicate
	return filterPorts(ports)
}

// filterPorts filters ports based on OS conventions
func filterPorts(ports []string) []string {
	var filtered []string
	seen := make(map[string]bool)

	for _, port := range ports {
		if seen[port] {
			continue
		}
		seen[port] = true

		// Always include TCP endpoints
		if strings.HasPrefix(port, "tcp://") {
			filtered = append(filtered, port)
			continue
		}

		// Windows: COM ports
		if runtime.GOOS == "windows" {
			if strings.HasPrefix(strings.ToUpper(port), "COM") {
				filtered = append(filtered, port)
			}
			continue
		}

		// macOS/Linux: filter by name
		lower := strings.ToLower(port)
		if strings.Contains(lower, "bluetooth") {
			continue
		}

		if strings.Contains(lower, "ttyusb") ||
			strings.Contains(lower, "ttyacm") ||
			strings.Contains(lower, "usbserial") ||
			strings.Contains(lower, "cu.") ||
			strings.Contains(lower, "ttys") {
			filtered = append(filtered, port)
		}
	}

	return filtered
}

// probePort performs ECHO handshake to verify POS device
func (s *Scanner) probePort(portName string) bool {
	// 1. Open Port
	port, err := OpenSerial(portName, 115200)
	if err != nil {
		logger.Debug("Failed to open %s: %v", portName, err)
		return false
	}
	defer port.Close()

	// 2. Clear buffer
	port.ResetInputBuffer()

	// 3. Send ECHO Request (TransType=80)
	req := protocol.ECPayRequest{
		TransType: "80",
		HostID:    "01",
	}
	packet := protocol.BuildPacket(req)

	logger.Debug("Sending ECHO to %s", portName)
	if _, err := port.Write(packet); err != nil {
		logger.Debug("Write failed on %s: %v", portName, err)
		return false
	}

	// 4. Wait for ACK (500ms)
	if !s.waitForACK(port, 500*time.Millisecond) {
		logger.Debug("No ACK from %s", portName)
		return false
	}
	logger.Debug("ACK received from %s", portName)

	// 5. Wait for Response (3s for probe)
	responsePacket, err := s.waitForResponse(port, 3*time.Second)
	if err != nil {
		logger.Debug("No response from %s: %v", portName, err)
		return false
	}

	// 6. Validate packet
	if !protocol.ValidatePacket(responsePacket) {
		logger.Debug("Invalid packet from %s", portName)
		return false
	}

	// 7. Verify ECHO response
	result := protocol.ParseResponse(responsePacket)
	if result["TransType"] != "80" {
		logger.Debug("Not ECHO response from %s: %s", portName, result["TransType"])
		return false
	}

	// 8. Send ACK
	port.Write([]byte{protocol.ACK})

	logger.Info("ECHO handshake successful on %s", portName)

	// 9. Close and reconnect via Manager
	port.Close()
	return s.Manager.ConnectTo(portName)
}

func (s *Scanner) waitForACK(port Port, timeout time.Duration) bool {
	buf := make([]byte, 64)
	start := time.Now()

	for time.Since(start) < timeout {
		n, _ := port.Read(buf)
		for i := 0; i < n; i++ {
			if buf[i] == protocol.ACK {
				return true
			}
			if buf[i] == protocol.NAK {
				logger.Debug("Received NAK")
				return false
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

func (s *Scanner) waitForResponse(port Port, timeout time.Duration) ([]byte, error) {
	buf := make([]byte, 1024)
	var respBuf []byte
	start := time.Now()

	for time.Since(start) < timeout {
		n, _ := port.Read(buf)
		if n > 0 {
			respBuf = append(respBuf, buf[:n]...)

			// Look for complete 603-byte packet
			if len(respBuf) >= 603 {
				for i := 0; i <= len(respBuf)-603; i++ {
					if respBuf[i] == protocol.STX {
						return respBuf[i : i+603], nil
					}
				}
			}
		}
		time.Sleep(20 * time.Millisecond)
	}

	return nil, fmt.Errorf("timeout")
}
