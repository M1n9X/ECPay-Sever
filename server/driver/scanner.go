package driver

import (
	"ecpay-server/logger"
	"ecpay-server/protocol"
	"fmt"
	"os"
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

	// Get all candidate ports
	ports := s.discoverPorts()

	if len(ports) == 0 {
		logger.Info("No serial ports found")
		return false
	}

	logger.Debug("Found %d candidate ports: %v", len(ports), ports)

	// Probe each port with ECHO handshake
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

// discoverPorts finds all candidate serial ports
func (s *Scanner) discoverPorts() []string {
	var ports []string

	// 1. Get hardware serial ports from OS
	hwPorts, err := serial.GetPortsList()
	if err != nil {
		logger.Error("Failed to list serial ports: %v", err)
	} else {
		ports = append(ports, hwPorts...)
	}

	// 2. Add virtual serial ports (PTY symlinks for development)
	if runtime.GOOS != "windows" {
		virtualPorts := []string{
			"/tmp/mock-pos-pty",   // Default mock-pos PTY location
			"/tmp/virtual-serial", // Alternative location
		}
		for _, vp := range virtualPorts {
			if _, err := os.Stat(vp); err == nil {
				ports = append(ports, vp)
			}
		}
	}

	// 3. Filter ports
	return filterPorts(ports)
}

// filterPorts filters serial ports based on OS-specific naming conventions
func filterPorts(ports []string) []string {
	var filtered []string
	seen := make(map[string]bool)

	for _, port := range ports {
		// Deduplicate
		if seen[port] {
			continue
		}
		seen[port] = true

		// Windows: COM ports
		if runtime.GOOS == "windows" {
			if strings.HasPrefix(strings.ToUpper(port), "COM") {
				filtered = append(filtered, port)
			}
			continue
		}

		// macOS/Linux
		lower := strings.ToLower(port)

		// Skip Bluetooth ports
		if strings.Contains(lower, "bluetooth") {
			continue
		}

		// Include:
		// - USB-Serial adapters
		// - Virtual PTY ports
		// - Standard serial ports
		if strings.Contains(lower, "ttyusb") ||
			strings.Contains(lower, "ttyacm") ||
			strings.Contains(lower, "usbserial") ||
			strings.Contains(lower, "cu.") ||
			strings.Contains(lower, "ttys") ||
			strings.HasPrefix(port, "/tmp/") {
			filtered = append(filtered, port)
		}
	}

	return filtered
}

// probePort attempts to connect and perform ECHO handshake to verify POS device
// Returns true only if:
// 1. Port can be opened
// 2. ECHO command (TransType=80) is sent successfully
// 3. ACK is received within timeout
// 4. Response packet is received and validated
func (s *Scanner) probePort(portName string) bool {
	// 1. Open Port
	port, err := OpenSerial(portName, 115200)
	if err != nil {
		logger.Debug("Failed to open %s: %v", portName, err)
		return false
	}
	defer port.Close()

	// 2. Clear input buffer
	if err := port.ResetInputBuffer(); err != nil {
		logger.Debug("Failed to reset buffer on %s: %v", portName, err)
	}

	// 3. Build and send ECHO Request (TransType=80)
	req := protocol.ECPayRequest{
		TransType: "80", // ECHO - connection test
		HostID:    "01", // Credit Card
	}
	packet := protocol.BuildPacket(req)

	logger.Debug("Sending ECHO to %s (%d bytes)", portName, len(packet))
	if _, err := port.Write(packet); err != nil {
		logger.Debug("Failed to write to %s: %v", portName, err)
		return false
	}

	// 4. Wait for ACK (500ms timeout for probing)
	if !s.waitForACK(port, 500*time.Millisecond) {
		logger.Debug("No ACK from %s", portName)
		return false
	}
	logger.Debug("ACK received from %s", portName)

	// 5. Wait for ECHO Response (2s timeout for probing)
	// Real POS should respond quickly to ECHO
	responsePacket, err := s.waitForResponse(port, 2*time.Second)
	if err != nil {
		logger.Debug("No response from %s: %v", portName, err)
		return false
	}

	// 6. Validate response packet
	if !protocol.ValidatePacket(responsePacket) {
		logger.Debug("Invalid response packet from %s", portName)
		return false
	}

	// 7. Parse and verify it's an ECHO response
	result := protocol.ParseResponse(responsePacket)
	if result["TransType"] != "80" {
		logger.Debug("Unexpected TransType from %s: %s", portName, result["TransType"])
		return false
	}

	// 8. Check response code (0000 = success)
	if result["RespCode"] != "0000" {
		logger.Debug("ECHO failed on %s: RespCode=%s", portName, result["RespCode"])
		// Still consider it a valid POS device, just might be busy
	}

	// 9. Send ACK to complete handshake
	port.Write([]byte{protocol.ACK})

	logger.Info("ECHO handshake successful on %s", portName)

	// 10. Close probe connection and let Manager open it
	port.Close()

	return s.Manager.ConnectTo(portName)
}

// waitForACK waits for ACK byte within timeout
func (s *Scanner) waitForACK(port Port, timeout time.Duration) bool {
	buf := make([]byte, 64)
	start := time.Now()

	for time.Since(start) < timeout {
		n, _ := port.Read(buf)
		if n > 0 {
			for i := 0; i < n; i++ {
				if buf[i] == protocol.ACK {
					return true
				}
				if buf[i] == protocol.NAK {
					// NAK means device responded but rejected
					logger.Debug("Received NAK (device present but rejected)")
					return false
				}
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

// waitForResponse waits for complete response packet within timeout
func (s *Scanner) waitForResponse(port Port, timeout time.Duration) ([]byte, error) {
	buf := make([]byte, 1024)
	var respBuf []byte
	start := time.Now()

	for time.Since(start) < timeout {
		n, _ := port.Read(buf)
		if n > 0 {
			respBuf = append(respBuf, buf[:n]...)

			// Check for complete packet (603 bytes: STX + 600 DATA + ETX + LRC)
			if len(respBuf) >= 603 {
				// Find STX
				for i := 0; i <= len(respBuf)-603; i++ {
					if respBuf[i] == protocol.STX {
						return respBuf[i : i+603], nil
					}
				}
			}
		}
		time.Sleep(20 * time.Millisecond)
	}

	return nil, fmt.Errorf("timeout waiting for response")
}
