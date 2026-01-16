package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"sync"
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
	// Timing
	ProcessingDelayMs   int  // Base processing delay (card swipe simulation)
	ByteStreamDelay     bool // Enable byte-by-byte transmission delay
	RandomDelayVariance int  // Random variance in ms added to processing

	// Error simulation
	NAKProbability     float64 // Probability of sending NAK (0.0-1.0)
	TimeoutProbability float64 // Probability of not responding (simulate timeout)
	DeclineProbability float64 // Probability of declined transaction
	PartialSendChunks  int     // If > 1, send response in chunks

	// Protocol compliance
	WaitForFinalACK   bool // Wait for ACK after sending response
	FinalACKTimeoutMs int  // Timeout for final ACK

	// Verbose logging
	Verbose bool
}

var config SimulationConfig

func main() {
	// Parse command line flags
	flag.IntVar(&config.ProcessingDelayMs, "delay", 2000, "Processing delay in ms")
	flag.BoolVar(&config.ByteStreamDelay, "byte-delay", false, "Enable byte-level transmission delay")
	flag.IntVar(&config.RandomDelayVariance, "delay-variance", 500, "Random delay variance in ms")
	flag.Float64Var(&config.NAKProbability, "nak-prob", 0.0, "Probability of NAK response (0.0-1.0)")
	flag.Float64Var(&config.TimeoutProbability, "timeout-prob", 0.0, "Probability of timeout (0.0-1.0)")
	flag.Float64Var(&config.DeclineProbability, "decline-prob", 0.0, "Probability of declined transaction (0.0-1.0)")
	flag.IntVar(&config.PartialSendChunks, "chunks", 1, "Number of chunks to split response into")
	flag.BoolVar(&config.WaitForFinalACK, "wait-ack", true, "Wait for final ACK from client")
	flag.IntVar(&config.FinalACKTimeoutMs, "ack-timeout", 3000, "Final ACK timeout in ms")
	flag.BoolVar(&config.Verbose, "verbose", false, "Enable verbose logging")
	flag.Parse()

	listener, err := net.Listen("tcp", ":9999")
	if err != nil {
		fmt.Println("Failed to start Mock POS:", err)
		return
	}
	defer listener.Close()

	printBanner()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Accept error:", err)
			continue
		}
		fmt.Printf("\n[MockPOS] Client connected: %s\n", conn.RemoteAddr())
		go handleConnection(conn)
	}
}

func printBanner() {
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║            Mock POS Simulator (RS232 over TCP)             ║")
	fmt.Println("╠════════════════════════════════════════════════════════════╣")
	fmt.Println("║  Listening on TCP :9999                                    ║")
	fmt.Println("╠════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Processing Delay  : %4d ms (±%d ms variance)             ║\n",
		config.ProcessingDelayMs, config.RandomDelayVariance)
	fmt.Printf("║  Byte Stream Delay : %-5v                                  ║\n", config.ByteStreamDelay)
	fmt.Printf("║  NAK Probability   : %.1f%%                                  ║\n", config.NAKProbability*100)
	fmt.Printf("║  Timeout Prob      : %.1f%%                                  ║\n", config.TimeoutProbability*100)
	fmt.Printf("║  Decline Prob      : %.1f%%                                  ║\n", config.DeclineProbability*100)
	fmt.Printf("║  Response Chunks   : %d                                      ║\n", config.PartialSendChunks)
	fmt.Printf("║  Wait Final ACK    : %-5v                                  ║\n", config.WaitForFinalACK)
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println("\nWaiting for connections...")
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	// Simulate serial port input buffer
	inputBuffer := &SerialBuffer{
		data: make([]byte, 0, 4096),
	}

	buf := make([]byte, 256) // Smaller reads to simulate serial behavior

	for {
		// Set read deadline to detect disconnection
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))

		n, err := conn.Read(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// Idle timeout, keep connection alive
				continue
			}
			fmt.Println("[MockPOS] Connection closed:", err)
			return
		}

		// Simulate bytes arriving over serial (with optional delay)
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

func processPacket(conn net.Conn, packet []byte) {
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

	// Simulate random NAK (for testing retry logic)
	if config.NAKProbability > 0 && rand.Float64() < config.NAKProbability {
		fmt.Println("[MockPOS] ✗ Simulating random NAK (test mode)")
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

	// Simulate processing delay (user swiping card, entering PIN, etc.)
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

	// Build response
	response := buildResponse(packet, declined)

	// Send response (possibly in chunks to simulate serial transmission)
	if config.PartialSendChunks > 1 {
		sendInChunks(conn, response, config.PartialSendChunks)
	} else {
		sendWithDelay(conn, response)
	}

	if declined {
		fmt.Println("[MockPOS] ✗ Response sent (DECLINED)")
	} else {
		fmt.Println("[MockPOS] ✓ Response sent (APPROVED)")
	}

	// Wait for final ACK from client (per RS232 spec)
	if config.WaitForFinalACK {
		waitForFinalACK(conn)
	}
}

func waitForFinalACK(conn net.Conn) {
	conn.SetReadDeadline(time.Now().Add(time.Duration(config.FinalACKTimeoutMs) * time.Millisecond))

	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if err != nil {
		logVerbose("[MockPOS] Final ACK timeout or error: %v", err)
		return
	}

	for i := 0; i < n; i++ {
		if buf[i] == ACK {
			fmt.Println("[MockPOS] ✓ Received final ACK from client")
			return
		}
	}
	logVerbose("[MockPOS] Did not receive ACK in response")
}

func sendWithDelay(conn net.Conn, data []byte) {
	if config.ByteStreamDelay {
		// Simulate byte-by-byte transmission at baud rate
		for _, b := range data {
			conn.Write([]byte{b})
			time.Sleep(time.Duration(ByteDelayMicros) * time.Microsecond)
		}
	} else {
		conn.Write(data)
	}
}

func sendInChunks(conn net.Conn, data []byte, chunks int) {
	chunkSize := len(data) / chunks
	for i := 0; i < chunks; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if i == chunks-1 {
			end = len(data) // Last chunk gets remainder
		}

		logVerbose("[MockPOS] Sending chunk %d/%d (%d bytes)", i+1, chunks, end-start)
		sendWithDelay(conn, data[start:end])

		// Small delay between chunks
		if i < chunks-1 {
			time.Sleep(50 * time.Millisecond)
		}
	}
}

// SerialBuffer simulates a serial port input buffer
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

	// Find STX
	stxIdx := bytes.IndexByte(sb.data, STX)
	if stxIdx < 0 {
		// No STX found, discard garbage
		sb.data = sb.data[:0]
		return nil
	}

	// Discard bytes before STX
	if stxIdx > 0 {
		logVerbose("[MockPOS] Discarding %d garbage bytes before STX", stxIdx)
		sb.data = sb.data[stxIdx:]
	}

	// Check if we have complete packet
	if len(sb.data) < 603 {
		return nil
	}

	// Extract packet
	packet := make([]byte, 603)
	copy(packet, sb.data[:603])
	sb.data = sb.data[603:]

	return packet
}

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
		"01": "SALE",
		"02": "REFUND",
		"10": "PREAUTH",
		"11": "AUTH_COMPLETE",
		"50": "SETTLEMENT",
		"80": "ECHO",
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
	if len(packet) != 603 {
		return false
	}
	if packet[0] != STX || packet[601] != ETX {
		return false
	}

	// Validate LRC: XOR of DATA + ETX
	payload := packet[1:602]
	recLrc := packet[602]
	calcLrc := calculateLRC(payload)

	logVerbose("[MockPOS] LRC check: received=0x%02X calculated=0x%02X", recLrc, calcLrc)
	return calcLrc == recLrc
}

func calculateLRC(data []byte) byte {
	var lrc byte = 0
	for _, b := range data {
		lrc ^= b
	}
	return lrc
}

func buildResponse(reqPacket []byte, declined bool) []byte {
	// Extract request data
	reqData := reqPacket[1:601]

	// Initialize response with spaces
	data := bytes.Repeat([]byte{0x20}, PacketLen)

	// Copy relevant fields from request
	copy(data[0:2], reqData[0:2])     // TransType
	copy(data[2:4], reqData[2:4])     // HostID
	copy(data[29:31], reqData[29:31]) // CUP Flag
	copy(data[31:43], reqData[31:43]) // Amount

	// Generate mock response fields
	now := time.Now()

	// Invoice Number (Offset 4, Len 6)
	copy(data[4:10], []byte(fmt.Sprintf("%06d", now.UnixNano()%1000000)))

	// Mock Card Number (Offset 10, Len 19) - masked
	cardNumbers := []string{
		"4311-****-****-1234",
		"5425-****-****-5678",
		"3530-****-****-9012",
		"6221-****-****-3456",
	}
	cardNo := cardNumbers[rand.Intn(len(cardNumbers))]
	copy(data[10:29], []byte(fmt.Sprintf("%-19s", cardNo)))

	// Trans Date (Offset 43, Len 6) - YYMMDD
	copy(data[43:49], []byte(now.Format("060102")))

	// Trans Time (Offset 49, Len 6) - hhmmss
	copy(data[49:55], []byte(now.Format("150405")))

	// Approval Number (Offset 55, Len 6)
	if declined {
		copy(data[55:61], []byte("      ")) // No approval for declined
	} else {
		copy(data[55:61], []byte(fmt.Sprintf("%06d", rand.Intn(1000000))))
	}

	// Response Code (Offset 61, Len 4)
	if declined {
		// Random decline reason
		declineCodes := []string{"0001", "0002", "0003"}
		copy(data[61:65], []byte(declineCodes[rand.Intn(len(declineCodes))]))
	} else {
		copy(data[61:65], []byte("0000")) // SUCCESS
	}

	// Terminal ID (Offset 65, Len 8)
	copy(data[65:73], []byte("TERM0001"))

	// Merchant ID (Offset 73, Len 15)
	copy(data[73:88], []byte("MER000123456789"))

	// EC Order Number (Offset 88, Len 20)
	orderNo := fmt.Sprintf("EC%s%04d", now.Format("20060102150405"), rand.Intn(10000))
	copy(data[88:108], []byte(fmt.Sprintf("%-20s", orderNo)))

	// Store ID (Offset 108, Len 18)
	copy(data[108:126], []byte(fmt.Sprintf("%-18s", "STORE001")))

	// Card Type (Offset 126, Len 2)
	cardTypes := []string{"00", "01", "02", "03"} // VISA, MC, JCB, CUP
	copy(data[126:128], []byte(cardTypes[rand.Intn(len(cardTypes))]))

	// POS Request Time - copy from request
	copy(data[492:506], reqData[492:506])

	// Request Hash - copy from request
	copy(data[506:546], reqData[506:546])

	// EDC Response Time (Offset 546, Len 14)
	copy(data[546:560], []byte(now.Format("20060102150405")))

	// Response Hash (Offset 560, Len 40)
	hashPayload := data[0:492]
	hash := sha1.Sum(hashPayload)
	hashHex := strings.ToUpper(hex.EncodeToString(hash[:]))
	copy(data[560:600], []byte(hashHex))

	// Build frame: STX + DATA + ETX + LRC
	frame := new(bytes.Buffer)
	frame.WriteByte(STX)
	frame.Write(data)
	frame.WriteByte(ETX)

	lrcPayload := append(data, ETX)
	lrc := calculateLRC(lrcPayload)
	frame.WriteByte(lrc)

	return frame.Bytes()
}

func logVerbose(format string, args ...interface{}) {
	if config.Verbose {
		fmt.Printf(format+"\n", args...)
	}
}
