package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
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
	STX byte = 0x02
	ETX byte = 0x03
	ACK byte = 0x06
	NAK byte = 0x15

	PayloadLen = 600
	FrameLen   = 603

	// Simulated baud rate delay (115200 bps ~ 14400 bytes/sec)
	ByteDelayMicros = 69
)

// SimulationConfig controls mock behavior
// This is a lightweight simulator for TCB RS232.
type SimulationConfig struct {
	Mode string

	TCPPort int

	ProcessingDelayMs   int
	ByteStreamDelay     bool
	RandomDelayVariance int

	NAKProbability     float64
	TimeoutProbability float64
	DeclineProbability float64

	WaitForFinalACK   bool
	FinalACKTimeoutMs int

	Verbose bool
}

var config SimulationConfig

func main() {
	rand.Seed(time.Now().UnixNano())

	flag.StringVar(&config.Mode, "mode", "tcp", "Connection mode: 'tcp'")
	flag.IntVar(&config.TCPPort, "port", 9999, "TCP port to listen on")
	flag.IntVar(&config.ProcessingDelayMs, "delay", 1200, "Processing delay in ms")
	flag.BoolVar(&config.ByteStreamDelay, "byte-delay", false, "Enable byte-level transmission delay")
	flag.IntVar(&config.RandomDelayVariance, "delay-variance", 400, "Random delay variance in ms")
	flag.Float64Var(&config.NAKProbability, "nak-prob", 0.0, "Probability of NAK response (0.0-1.0)")
	flag.Float64Var(&config.TimeoutProbability, "timeout-prob", 0.0, "Probability of timeout (0.0-1.0)")
	flag.Float64Var(&config.DeclineProbability, "decline-prob", 0.0, "Probability of declined transaction (0.0-1.0)")
	flag.BoolVar(&config.WaitForFinalACK, "wait-ack", true, "Wait for final ACK from client")
	flag.IntVar(&config.FinalACKTimeoutMs, "ack-timeout", 3000, "Final ACK timeout in ms")
	flag.BoolVar(&config.Verbose, "verbose", false, "Enable verbose logging")
	flag.Parse()

	printBanner()

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
	fmt.Println("===============================================")
	fmt.Println("  Mock POS Simulator (TCB RS232) ")
	fmt.Println("===============================================")
	fmt.Printf("  Mode: TCP\n")
	fmt.Printf("  Listen Port: %d\n", config.TCPPort)
	fmt.Printf("  Processing Delay: %d ms (±%d)\n", config.ProcessingDelayMs, config.RandomDelayVariance)
	fmt.Printf("  NAK Probability  : %.1f%%\n", config.NAKProbability*100)
	fmt.Printf("  Timeout Prob     : %.1f%%\n", config.TimeoutProbability*100)
	fmt.Printf("  Decline Prob     : %.1f%%\n", config.DeclineProbability*100)
	fmt.Println("===============================================")

	if runtime.GOOS != "windows" {
		fmt.Println("NOTE: For development, Server should connect via TCP.")
		fmt.Println("      Use: server_tcb with tcp://localhost:9999")
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

	inputBuffer := &SerialBuffer{data: make([]byte, 0, 4096)}
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

		if config.ByteStreamDelay {
			for i := 0; i < n; i++ {
				inputBuffer.Write(buf[i : i+1])
				time.Sleep(time.Duration(ByteDelayMicros) * time.Microsecond)
			}
		} else {
			inputBuffer.Write(buf[:n])
		}

		logVerbose("[MockPOS] Received %d bytes, buffer size: %d", n, inputBuffer.Len())

		for {
			packet := inputBuffer.ExtractPacket()
			if packet == nil {
				break
			}
			processPacket(conn, packet)
		}
	}
}

// ============================================================================
// Serial Buffer
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

	if len(sb.data) < FrameLen {
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

	if len(sb.data) < FrameLen {
		return nil
	}

	packet := make([]byte, FrameLen)
	copy(packet, sb.data[:FrameLen])
	sb.data = sb.data[FrameLen:]

	return packet
}

// ============================================================================
// Protocol Implementation
// ============================================================================

func processPacket(conn Connection, packet []byte) {
	if !validatePacket(packet) {
		logVerbose("[MockPOS] Invalid packet LRC/length")
		writeNAK(conn)
		return
	}

	if rand.Float64() < config.NAKProbability {
		logVerbose("[MockPOS] Simulating NAK")
		writeNAK(conn)
		return
	}

	writeACK(conn)

	if rand.Float64() < config.TimeoutProbability {
		logVerbose("[MockPOS] Simulating timeout (no response)")
		return
	}

	processingDelay := config.ProcessingDelayMs
	if config.RandomDelayVariance > 0 {
		processingDelay += rand.Intn(config.RandomDelayVariance)
	}
	time.Sleep(time.Duration(processingDelay) * time.Millisecond)

	declined := rand.Float64() < config.DeclineProbability
	response := buildResponse(packet, declined)

	sendResponse(conn, response)

	if config.WaitForFinalACK {
		if !waitForFinalACK(conn, time.Duration(config.FinalACKTimeoutMs)*time.Millisecond) {
			logVerbose("[MockPOS] Final ACK timeout, resending response once")
			sendResponse(conn, response)
			_ = waitForFinalACK(conn, time.Duration(config.FinalACKTimeoutMs)*time.Millisecond)
		}
	}
}

func sendResponse(conn Connection, response []byte) {
	if config.ByteStreamDelay {
		for _, b := range response {
			_, _ = conn.Write([]byte{b})
			time.Sleep(time.Duration(ByteDelayMicros) * time.Microsecond)
		}
		return
	}
	_, _ = conn.Write(response)
}

func waitForFinalACK(conn Connection, timeout time.Duration) bool {
	buf := make([]byte, 64)
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		n, err := conn.Read(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			return false
		}
		if n == 0 {
			continue
		}
		for i := 0; i < n; i++ {
			if buf[i] == ACK {
				return true
			}
		}
	}
	return false
}

func writeACK(conn Connection) {
	_, _ = conn.Write([]byte{ACK, ACK})
}

func writeNAK(conn Connection) {
	_, _ = conn.Write([]byte{NAK, NAK})
}

func validatePacket(packet []byte) bool {
	if len(packet) != FrameLen || packet[0] != STX || packet[601] != ETX {
		return false
	}
	payload := packet[1:602]
	return calculateLRC(payload) == packet[602]
}

func calculateLRC(data []byte) byte {
	var lrc byte
	for _, b := range data {
		lrc ^= b
	}
	return lrc
}

func buildResponse(reqPacket []byte, declined bool) []byte {
	reqData := reqPacket[1 : 1+PayloadLen]
	transType := strings.TrimSpace(string(reqData[0:2]))

	layout, ok := getLayout(transType)
	fields := map[string]string{}
	if ok {
		fields = parseFields(reqData, layout)
	}

	data := bytes.Repeat([]byte(" "), PayloadLen)

	hostID := strings.TrimSpace(string(reqData[2:4]))
	if v := fields["Host_ID"]; v != "" {
		hostID = v
	}
	if hostID == "" {
		hostID = "02"
	}

	amount := fields["Trans_Amount"]

	ecrCode := "0000"
	hostResp := "00"
	if declined {
		ecrCode = "0001"
		hostResp = "05"
	}

	if ok {
		writeFieldByName(data, layout, "Trans_Type", transType)
		writeFieldByName(data, layout, "Host_ID", hostID)
		if amount != "" {
			writeFieldByName(data, layout, "Trans_Amount", amount)
		}

		for _, f := range layout.Fields {
			if strings.Contains(f.Field, "ECR_Response_Code") {
				writeField(data, f.Pos-1, f.Len, ecrCode, padTypeForField(f.Field))
			}
			if f.Field == "Host_Response_Code" || f.Field == "Host_Resp_Code" {
				writeField(data, f.Pos-1, f.Len, hostResp, padTypeForField(f.Field))
			}
		}
	} else {
		// Fallback positions
		copy(data[0:2], []byte(fmt.Sprintf("%-2s", transType)))
		copy(data[2:4], []byte(fmt.Sprintf("%-2s", hostID)))
		if amount != "" {
			copy(data[4:16], []byte(fmt.Sprintf("%012s", amount)))
		}
	}

	frame := make([]byte, 0, FrameLen)
	frame = append(frame, STX)
	frame = append(frame, data...)
	frame = append(frame, ETX)
	lrc := calculateLRC(append(data, ETX))
	frame = append(frame, lrc)
	return frame
}

func parseFields(data []byte, layout LayoutDef) map[string]string {
	result := make(map[string]string)
	for _, f := range layout.Fields {
		start := f.Pos - 1
		end := start + f.Len
		if start < 0 || end > len(data) {
			continue
		}
		val := strings.TrimRight(string(data[start:end]), " ")
		result[f.Field] = val
	}
	return result
}

func writeFieldByName(buf []byte, layout LayoutDef, fieldName, val string) bool {
	for _, f := range layout.Fields {
		if f.Field == fieldName {
			writeField(buf, f.Pos-1, f.Len, val, padTypeForField(f.Field))
			return true
		}
	}
	return false
}

func writeField(buf []byte, offset, length int, val string, pad string) {
	if offset < 0 || offset+length > len(buf) {
		return
	}
	if len(val) > length {
		val = val[:length]
	}
	var field string
	if pad == "LEFT_ZERO" {
		field = fmt.Sprintf("%0*s", length, val)
	} else {
		field = fmt.Sprintf("%-*s", length, val)
	}
	copy(buf[offset:offset+length], []byte(field))
}

func padTypeForField(field string) string {
	field = strings.TrimSpace(field)
	if strings.Contains(field, "ECR_Response_Code") {
		return "LEFT_ZERO"
	}
	if field == "Host_Response_Code" || field == "Host_Resp_Code" {
		return "LEFT_ZERO"
	}
	if numericFields[field] {
		return "LEFT_ZERO"
	}
	return "RIGHT_SPACE"
}

var numericFields = map[string]bool{
	"Trans_Type":          true,
	"Host_ID":             true,
	"CardType":            true,
	"Card_Type":           true,
	"Issuer Id":           true,
	"Issuer_ID":           true,
	"IssuerId":            true,
	"Start Get PAN":       true,
	"Only_Credit/CUP":     true,
	"WaveFlag":            true,
	"Order_Status":        true,
	"ProcessStatus":       true,
	"ECR_Response_Code":   true,
	"Host_Response_Code":  true,
	"Host_Resp_Code":      true,
	"NP_Resp_Errcode":     true,
	"Trans_Amount":        true,
	"Auth_Amount":         true,
	"DownPayment":         true,
	"EachPayment":         true,
	"FeeAmt":              true,
	"Redeem_Point_Remain": true,
	"Redeem_Point_Cost":   true,
	"Redeem_ActNum":       true,
	"Redeem_Config":       true,
	"Redeem_Response":     true,
	"Amount_After_Cost":   true,
	"Sale_Amount":         true,
	"Refund_Amount":       true,
	"Sale_Count":          true,
	"Refund_Count":        true,
	"Period":              true,
	"Trans_Date":          true,
	"Trans_Time":          true,
	"Old Trans_Time":      true,
	"Org Trans_Date":      true,
	"Trans_DateTime":      true,
}

// ============================================================================
// Layout (embedded JSON)
// ============================================================================

type FieldDef struct {
	Pos   int    `json:"pos"`
	Len   int    `json:"len"`
	Field string `json:"field"`
	Desc  string `json:"desc"`
}

type LayoutDef struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	TransTypes []string   `json:"trans_types,omitempty"`
	Notes      string     `json:"notes,omitempty"`
	Fields     []FieldDef `json:"fields"`
}

type Spec struct {
	Layouts []LayoutDef `json:"layouts"`
}

//go:embed tcb_layout_v36.json
var layoutJSON []byte

var (
	layoutOnce sync.Once
	layoutSpec Spec
	layoutErr  error
	layoutByTT map[string]LayoutDef
)

func loadLayouts() (Spec, error) {
	layoutOnce.Do(func() {
		if err := json.Unmarshal(layoutJSON, &layoutSpec); err != nil {
			layoutErr = err
			return
		}
		layoutByTT = make(map[string]LayoutDef)
		for _, l := range layoutSpec.Layouts {
			for _, tt := range l.TransTypes {
				layoutByTT[tt] = l
			}
		}
	})
	return layoutSpec, layoutErr
}

func getLayout(transType string) (LayoutDef, bool) {
	_, err := loadLayouts()
	if err != nil {
		return LayoutDef{}, false
	}
	l, ok := layoutByTT[transType]
	return l, ok
}

func logVerbose(format string, args ...interface{}) {
	if !config.Verbose {
		return
	}
	fmt.Printf(format+"\n", args...)
}
