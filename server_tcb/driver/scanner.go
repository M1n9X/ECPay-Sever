package driver

import (
	"tcb-server/logger"
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

// probePort attempts to open a port. TCB does not define an ECHO handshake.
// If auto-detect is enabled, we connect to the first port that opens successfully.
func (s *Scanner) probePort(portName string) bool {
	// 1. Open Port
	port, err := OpenSerial(portName, s.Manager.Baud)
	if err != nil {
		logger.Debug("Failed to open %s: %v", portName, err)
		return false
	}
	defer port.Close()

	// 2. Clear buffer
	port.ResetInputBuffer()

	// 3. Close and reconnect via Manager
	port.Close()
	return s.Manager.ConnectTo(portName)
}
