package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	STX       byte = 0x02
	ETX       byte = 0x03
	ACK       byte = 0x06
	NAK       byte = 0x15
	PacketLen      = 600

	// Simulated baud rate delay (115200 bps = ~14400 bytes/sec)
	ByteDelayMicros = 69 // ~1/14400 seconds per byte
)

// SimulationConfig controls mock behavior
type SimulationConfig struct {
	// Connection mode
	Mode    string // "tcp" only for now (PTY requires platform-specific code)
	TCPPort int    // TCP port (default 9999)

	// Timing
	ProcessingDelayMs   int  // Base processing delay (card swipe simulation)
	ByteStreamDelay     bool // Enable byte-by-byte transmission delay
	RandomDelayVariance int  // Random variance in ms added to processing

	// Error simulation
	NAKProbability     float64 // Probability of sending NAK (0.0-1.0)
	TimeoutProbability float64 // Probability of not responding (simulate timeout)
	DeclineProbability float64 // Probability of declined transaction

	// Protocol compliance
	WaitForFinalACK   bool // Wait for ACK after sending response
	FinalACKTimeoutMs int  // Timeout for final ACK

	// Verbose logging
	Verbose bool
}

var config SimulationConfig

func main() {
	// Parse command line flags
	flag.StringVar(&config.Mode, "mode", "tcp", "Connection mode: 'tcp'")
	flag.IntVar(&config.TCPPort, "port", 9999, "TCP port to listen on")
	flag.IntVar(&config.ProcessingDelayMs, "delay", 2000, "Processing delay in ms")
	flag.BoolVar(&config.ByteStreamDelay, "byte-delay", false, "Enable byte-level transmission delay")
	flag.IntVar(&config.RandomDelayVariance, "delay-variance", 500, "Random delay variance in ms")
	flag.Float64Var(&config.NAKProbability, "nak-prob", 0.0, "Probability of NAK response (0.0-1.0)")
	flag.Float64Var(&config.TimeoutProbability, "timeout-prob", 0.0, "Probability of timeout (0.0-1.0)")
	flag.Float64Var(&config.DeclineProbability, "decline-prob", 0.0, "Probability of declined transaction (0.0-1.0)")
	flag.BoolVar(&config.WaitForFinalACK, "wait-ack", true, "Wait for final ACK from client")
	flag.IntVar(&config.FinalACKTimeoutMs, "ack-timeout", 3000, "Final ACK timeout in ms")
	flag.BoolVar(&config.Verbose, "verbose", false, "Enable verbose logging")
	flag.Parse()

	printBanner()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n[MockPOS] Shutting down...")
		os.Exit(0)
	}()

	runTCPMode()
}

func printBanner() {
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║              Mock POS Simulator (ECPay RS232)              ║")
	fmt.Println("╠════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Mode: TCP                                                  ║\n")
	fmt.Printf("║  Listen Port: %-45d ║\n", config.TCPPort)
	fmt.Println("╠════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Processing Delay  : %4d ms (±%d ms)                       ║\n",
		config.ProcessingDelayMs, config.RandomDelayVariance)
	fmt.Printf("║  NAK Probability   : %5.1f%%                                 ║\n", config.NAKProbability*100)
	fmt.Printf("║  Timeout Prob      : %5.1f%%                                 ║\n", config.TimeoutProbability*100)
	fmt.Printf("║  Decline Prob      : %5.1f%%                                 ║\n", config.DeclineProbability*100)
	fmt.Println("╚════════════════════════════════════════════════════════════╝")

	if runtime.GOOS != "windows" {
		fmt.Println("")
		fmt.Println("NOTE: For development, Server should connect via TCP.")
		fmt.Println("      Use: ./ecpay-server (with scanner finding tcp://localhost:9999)")
	}
}

// ============================================================================
// TCP Mode
// ============================================================================

func runTCPMode() {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", config.TCPPort))
	if err != nil {
		fmt.Printf("Failed to start TCP listener: %v\n", err)
		os.Exit(1)
	}
	defer listener.Close()

	fmt.Printf("\n[MockPOS] Listening on TCP :%d\n", config.TCPPort)
	fmt.Println("[MockPOS] Waiting for connections...")

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Accept error:", err)
			continue
		}
		fmt.Printf("\n[MockPOS] Client connected: %s\n", conn.RemoteAddr())
		go handleConnection(&tcpConn{conn})
	}
}

type tcpConn struct {
	net.Conn
}

func (t *tcpConn) Read(p []byte) (int, error) {
	t.Conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	return t.Conn.Read(p)
}

// ============================================================================
// Connection Handler
// ============================================================================

type Connection interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	Close() error
}

func handleConnection(conn Connection) {
	defer conn.Close()

	// Simulate serial port input buffer
	inputBuffer := &SerialBuffer{
		data: make([]byte, 0, 4096),
	}

	buf := make([]byte, 256)

	for {
		n, err := conn.Read(buf)
		if err != nil {
			if err == io.EOF {
				fmt.Println("[MockPOS] Connection closed (EOF)")
				return
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			fmt.Printf("[MockPOS] Read error: %v\n", err)
			return
		}

		if n == 0 {
			continue
		}

		// Simulate bytes arriving over serial
		if config.ByteStreamDelay {
			for i := 0; i < n; i++ {
				inputBuffer.Write(buf[i : i+1])
				time.Sleep(time.Duration(ByteDelayMicros) * time.Microsecond)
			}
		} else {
			inputBuffer.Write(buf[:n])
		}

		logVerbose("[MockPOS] Received %d bytes, buffer size: %d", n, inputBuffer.Len())

		// Try to extract complete packet
		packet := inputBuffer.ExtractPacket()
		if packet == nil {
			continue
		}

		// Process the packet
		processPacket(conn, packet)
	}
}

func processPacket(conn Connection, packet []byte) {
	logVerbose("[MockPOS] Complete packet received (603 bytes)")

	// Parse request info for logging
	reqInfo := parseRequestInfo(packet)
	fmt.Printf("[MockPOS] Request: Type=%s Amount=%s\n", reqInfo.TransType, reqInfo.Amount)

	// Validate packet (LRC check)
	if !validatePacket(packet) {
		fmt.Println("[MockPOS] ✗ Invalid LRC checksum. Sending NAK.")
		sendWithDelay(conn, []byte{NAK})
		return
	}

	// Simulate random NAK
	if config.NAKProbability > 0 && rand.Float64() < config.NAKProbability {
		fmt.Println("[MockPOS] ✗ Simulating random NAK")
		sendWithDelay(conn, []byte{NAK})
		return
	}

	// Send ACK
	fmt.Println("[MockPOS] ✓ Valid packet. Sending ACK...")
	sendWithDelay(conn, []byte{ACK})

	// Simulate timeout (no response)
	if config.TimeoutProbability > 0 && rand.Float64() < config.TimeoutProbability {
		fmt.Println("[MockPOS] ⏱ Simulating timeout (no response)")
		return
	}

	// Simulate processing delay
	delay := config.ProcessingDelayMs
	if config.RandomDelayVariance > 0 {
		delay += rand.Intn(config.RandomDelayVariance*2) - config.RandomDelayVariance
		if delay < 500 {
			delay = 500
		}
	}
	fmt.Printf("[MockPOS] Processing transaction (%d ms delay)...\n", delay)
	time.Sleep(time.Duration(delay) * time.Millisecond)

	// Determine if transaction should be declined
	declined := config.DeclineProbability > 0 && rand.Float64() < config.DeclineProbability

	// Build and send response
	response := buildResponse(packet, declined)
	sendWithDelay(conn, response)

	if declined {
		fmt.Println("[MockPOS] ✗ Response sent (DECLINED)")
	} else {
		fmt.Println("[MockPOS] ✓ Response sent (APPROVED)")
	}

	// Wait for final ACK
	if config.WaitForFinalACK {
		waitForFinalACK(conn)
	}
}

func waitForFinalACK(conn Connection) {
	buf := make([]byte, 64)
	deadline := time.Now().Add(time.Duration(config.FinalACKTimeoutMs) * time.Millisecond)

	for time.Now().Before(deadline) {
		n, err := conn.Read(buf)
		if err != nil {
			logVerbose("[MockPOS] Final ACK read error: %v", err)
			return
		}
		for i := 0; i < n; i++ {
			if buf[i] == ACK {
				fmt.Println("[MockPOS] ✓ Received final ACK")
				return
			}
		}
	}
	logVerbose("[MockPOS] Final ACK timeout")
}

func sendWithDelay(conn Connection, data []byte) {
	if config.ByteStreamDelay {
		for _, b := range data {
			conn.Write([]byte{b})
			time.Sleep(time.Duration(ByteDelayMicros) * time.Microsecond)
		}
	} else {
		conn.Write(data)
	}
}

// ============================================================================
// Serial Buffer (simulates hardware buffer)
// ============================================================================

type SerialBuffer struct {
	mu   sync.Mutex
	data []byte
}

func (sb *SerialBuffer) Write(p []byte) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	sb.data = append(sb.data, p...)
}

func (sb *SerialBuffer) Len() int {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return len(sb.data)
}

func (sb *SerialBuffer) ExtractPacket() []byte {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	if len(sb.data) < 603 {
		return nil
	}

	stxIdx := bytes.IndexByte(sb.data, STX)
	if stxIdx < 0 {
		sb.data = sb.data[:0]
		return nil
	}

	if stxIdx > 0 {
		logVerbose("[MockPOS] Discarding %d garbage bytes", stxIdx)
		sb.data = sb.data[stxIdx:]
	}

	if len(sb.data) < 603 {
		return nil
	}

	packet := make([]byte, 603)
	copy(packet, sb.data[:603])
	sb.data = sb.data[603:]

	return packet
}

// ============================================================================
// Protocol Implementation
// ============================================================================

type RequestInfo struct {
	TransType string
	HostID    string
	Amount    string
	OrderNo   string
}

func parseRequestInfo(packet []byte) RequestInfo {
	if len(packet) < 603 {
		return RequestInfo{}
	}
	data := packet[1:601]

	readField := func(offset, length int) string {
		return strings.TrimSpace(string(data[offset : offset+length]))
	}

	transType := readField(0, 2)
	transName := map[string]string{
		"01": "SALE", "02": "REFUND", "10": "PREAUTH",
		"11": "AUTH_COMPLETE", "50": "SETTLEMENT", "80": "ECHO",
	}[transType]
	if transName == "" {
		transName = transType
	}

	return RequestInfo{
		TransType: transName,
		HostID:    readField(2, 2),
		Amount:    readField(31, 12),
		OrderNo:   readField(88, 20),
	}
}

func validatePacket(packet []byte) bool {
	if len(packet) != 603 || packet[0] != STX || packet[601] != ETX {
		return false
	}
	payload := packet[1:602]
	return calculateLRC(payload) == packet[602]
}

func calculateLRC(data []byte) byte {
	var lrc byte = 0
	for _, b := range data {
		lrc ^= b
	}
	return lrc
}

func buildResponse(reqPacket []byte, declined bool) []byte {
	reqData := reqPacket[1:601]
	data := bytes.Repeat([]byte{0x20}, PacketLen)

	// Copy from request
	copy(data[0:2], reqData[0:2])     // TransType
	copy(data[2:4], reqData[2:4])     // HostID
	copy(data[29:31], reqData[29:31]) // CUP Flag
	copy(data[31:43], reqData[31:43]) // Amount

	now := time.Now()

	// Invoice Number
	copy(data[4:10], []byte(fmt.Sprintf("%06d", now.UnixNano()%1000000)))

	// Card Number (masked)
	cards := []string{"4311-****-****-1234", "5425-****-****-5678", "3530-****-****-9012"}
	copy(data[10:29], []byte(fmt.Sprintf("%-19s", cards[rand.Intn(len(cards))])))

	// Date/Time
	copy(data[43:49], []byte(now.Format("060102")))
	copy(data[49:55], []byte(now.Format("150405")))

	// Approval Number
	if !declined {
		copy(data[55:61], []byte(fmt.Sprintf("%06d", rand.Intn(1000000))))
	}

	// Response Code
	if declined {
		codes := []string{"0001", "0002", "0003"}
		copy(data[61:65], []byte(codes[rand.Intn(len(codes))]))
	} else {
		copy(data[61:65], []byte("0000"))
	}

	// Terminal/Merchant
	copy(data[65:73], []byte("TERM0001"))
	copy(data[73:88], []byte("MER000123456789"))

	// Order Number
	orderNo := fmt.Sprintf("EC%s%04d", now.Format("20060102150405"), rand.Intn(10000))
	copy(data[88:108], []byte(fmt.Sprintf("%-20s", orderNo)))

	// Store ID
	copy(data[108:126], []byte(fmt.Sprintf("%-18s", "STORE001")))

	// Card Type
	cardTypes := []string{"00", "01", "02", "03"}
	copy(data[126:128], []byte(cardTypes[rand.Intn(len(cardTypes))]))

	// Copy request time/hash
	copy(data[492:506], reqData[492:506])
	copy(data[506:546], reqData[506:546])

	// EDC Response Time
	copy(data[546:560], []byte(now.Format("20060102150405")))

	// Response Hash
	hash := sha1.Sum(data[0:492])
	copy(data[560:600], []byte(strings.ToUpper(hex.EncodeToString(hash[:]))))

	// Build frame
	frame := new(bytes.Buffer)
	frame.WriteByte(STX)
	frame.Write(data)
	frame.WriteByte(ETX)
	frame.WriteByte(calculateLRC(append(data, ETX)))

	return frame.Bytes()
}

func logVerbose(format string, args ...interface{}) {
	if config.Verbose {
		fmt.Printf(format+"\n", args...)
	}
}
