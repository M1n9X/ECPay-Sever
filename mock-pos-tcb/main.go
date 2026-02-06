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

	// Realism toggles
	ReturnCard           bool
	ReturnInvoice        bool
	ReturnReference      bool
	ReturnApproval       bool
	RequireRefForFisc    bool
	RequireRefForInst    bool
	RequireCardMatch     bool
	RequireRefAndCard    bool
	RefundCardMode       string // "last" or "random"
	RequireBatchOrCancel bool
	MaxRefundAgeHours    int
	LoginSuccess         bool
}

var config SimulationConfig

type txRecord struct {
	TransType  string
	InvoiceNo  string
	Reference  string
	ApprovalNo string
	CardNo     string
	BatchNo    string
	CancelDebt string
	Amount     int
	Time       time.Time
}

type txStore struct {
	mu       sync.Mutex
	list     []txRecord
	refunded map[string]bool
}

func (s *txStore) add(rec txRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.refunded == nil {
		s.refunded = make(map[string]bool)
	}
	s.list = append(s.list, rec)
}

func (s *txStore) latest() *txRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.list) == 0 {
		return nil
	}
	rec := s.list[len(s.list)-1]
	return &rec
}

func (s *txStore) findByReference(ref string) *txRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.list) - 1; i >= 0; i-- {
		if s.list[i].Reference == ref {
			rec := s.list[i]
			return &rec
		}
	}
	return nil
}

func (s *txStore) isRefunded(ref string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.refunded == nil {
		return false
	}
	return s.refunded[ref]
}

func (s *txStore) markRefunded(ref string) {
	if ref == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.refunded == nil {
		s.refunded = make(map[string]bool)
	}
	s.refunded[ref] = true
}

func (s *txStore) findByInvoice(inv string) *txRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.list) - 1; i >= 0; i-- {
		if s.list[i].InvoiceNo == inv {
			rec := s.list[i]
			return &rec
		}
	}
	return nil
}

func (s *txStore) findByApproval(app string) *txRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.list) - 1; i >= 0; i-- {
		if s.list[i].ApprovalNo == app {
			rec := s.list[i]
			return &rec
		}
	}
	return nil
}

func (s *txStore) findByCardAndAmount(card string, amount int) *txRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.list) - 1; i >= 0; i-- {
		if s.list[i].CardNo == card && s.list[i].Amount >= amount {
			rec := s.list[i]
			return &rec
		}
	}
	return nil
}

func (s *txStore) randomCard() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.list) == 0 {
		return ""
	}
	rec := s.list[rand.Intn(len(s.list))]
	return rec.CardNo
}

func (s *txStore) findByBatchOrCancel(batch, cancel string) *txRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.list) - 1; i >= 0; i-- {
		if batch != "" && s.list[i].BatchNo == batch {
			rec := s.list[i]
			return &rec
		}
		if cancel != "" && s.list[i].CancelDebt == cancel {
			rec := s.list[i]
			return &rec
		}
	}
	return nil
}

var store txStore

type getPanState struct {
	Mode   string
	Amount int
	Time   time.Time
	CardNo string
	HostID string
}

var lastGetPan struct {
	mu sync.Mutex
	st *getPanState
}

func setGetPan(st *getPanState) {
	lastGetPan.mu.Lock()
	defer lastGetPan.mu.Unlock()
	lastGetPan.st = st
}

func getGetPan() *getPanState {
	lastGetPan.mu.Lock()
	defer lastGetPan.mu.Unlock()
	if lastGetPan.st == nil {
		return nil
	}
	cp := *lastGetPan.st
	return &cp
}

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
	flag.BoolVar(&config.ReturnCard, "return-card", true, "Return Card_No when available")
	flag.BoolVar(&config.ReturnInvoice, "return-invoice", true, "Return Invoice_No")
	flag.BoolVar(&config.ReturnReference, "return-reference", true, "Return Reference_No")
	flag.BoolVar(&config.ReturnApproval, "return-approval", true, "Return Approval_No")
	flag.BoolVar(&config.RequireRefForFisc, "require-ref-fisc", true, "Require reference fields for FISC refund (27)")
	flag.BoolVar(&config.RequireRefForInst, "require-ref-inst", true, "Require reference fields for INST refund (04)")
	flag.BoolVar(&config.RequireCardMatch, "require-card-match", false, "Require card match on refund")
	flag.BoolVar(&config.RequireRefAndCard, "require-ref-and-card", false, "Require both reference fields and card match on refund")
	flag.StringVar(&config.RefundCardMode, "refund-card-mode", "last", "Refund card mode: last or random")
	flag.BoolVar(&config.RequireBatchOrCancel, "require-batch-or-cancel", false, "Require Batch_Number or Cancel Debt Number on refund (simulate device rule)")
	flag.IntVar(&config.MaxRefundAgeHours, "refund-max-age-hours", 0, "Max refund age (hours); older transactions will be rejected")
	flag.BoolVar(&config.LoginSuccess, "login-success", true, "User login (trans 92) returns success")
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
	response, diag := buildResponse(packet, declined)

	sendResponse(conn, response)

	if config.WaitForFinalACK {
		if !waitForFinalACK(conn, time.Duration(config.FinalACKTimeoutMs)*time.Millisecond) {
			logVerbose("[MockPOS] Final ACK timeout, resending response once")
			sendResponse(conn, response)
			_ = waitForFinalACK(conn, time.Duration(config.FinalACKTimeoutMs)*time.Millisecond)
		}
	}

	logDecision(diag)
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

func buildResponse(reqPacket []byte, declined bool) ([]byte, diagInfo) {
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
	now := time.Now()
	amountInt := parseAmount(amount)
	transDate := now.Format("060102")
	transTime := now.Format("150405")
	invoiceNo := strings.TrimSpace(fields["Invoice_No"])
	if invoiceNo == "" {
		invoiceNo = fmt.Sprintf("%06d", now.UnixNano()%1000000)
	}
	referenceNo := strings.TrimSpace(fields["Reference_No"])
	if referenceNo == "" {
		referenceNo = transDate + transTime
	}
	approvalNo := strings.TrimSpace(fields["Approval_No"])
	if approvalNo == "" {
		approvalNo = transTime
	}
	batchNo := strings.TrimSpace(fields["Batch_Number"])
	if batchNo == "" {
		batchNo = fmt.Sprintf("%06d", rand.Intn(1000000))
	}
	cancelDebt := strings.TrimSpace(fields["Cancel Debt Number"])
	if cancelDebt == "" {
		cancelDebt = fmt.Sprintf("%016d", rand.Intn(100000000))
	}
	cardNo := strings.TrimSpace(fields["Card_No"])
	if cardNo == "" {
		cards := []string{
			"524689******0179",
			"431112******1234",
			"542531******5678",
			"353012******9012",
		}
		cardNo = cards[rand.Intn(len(cards))]
	}
	cardType := strings.TrimSpace(fields["CardType"])
	if cardType == "" {
		cardType = strings.TrimSpace(fields["Card_Type"])
	}
	if cardType == "" {
		cardType = "2"
	}
	terminalID := "00010001"
	merchantID := "006006111110001"

	ecrCode := "0000"
	hostResp := "00"

	refReq := strings.TrimSpace(fields["Reference_No"])
	invReq := strings.TrimSpace(fields["Invoice_No"])
	appReq := strings.TrimSpace(fields["Approval_No"])
	reqCard := strings.TrimSpace(fields["Card_No"])
	batchReq := strings.TrimSpace(fields["Batch_Number"])
	cancelReq := strings.TrimSpace(fields["Cancel Debt Number"])
	startGetPan := strings.TrimSpace(fields["Start Get PAN"])

	var matched *txRecord
	diag := diagInfo{
		TransType: transType,
		Amount:    amount,
		RefReq:    refReq,
		InvReq:    invReq,
		AppReq:    appReq,
		ReqCard:   reqCard,
		BatchReq:  batchReq,
		CancelReq: cancelReq,
		StartGet:  startGetPan,
	}

	// 特殊交易: 使用者登入回傳 (92)
	if transType == "92" {
		if config.LoginSuccess {
			ecrCode = "0000"
			hostResp = "00"
		} else {
			ecrCode = "0001"
			hostResp = "05"
			diag.Reason = "登入失敗"
		}
	}

	// 特殊交易: 交易回傳 (91)
	if transType == "91" {
		var rec *txRecord
		if invReq != "" {
			rec = store.findByInvoice(invReq)
		} else {
			rec = store.latest()
		}
		if rec == nil {
			ecrCode = "0001"
			hostResp = "05"
			diag.Reason = "查無原交易"
		} else {
			invoiceNo = rec.InvoiceNo
			referenceNo = rec.Reference
			approvalNo = rec.ApprovalNo
			cardNo = rec.CardNo
			amountInt = rec.Amount
			amount = fmt.Sprintf("%012d", rec.Amount)
			transDate = rec.Time.Format("060102")
			transTime = rec.Time.Format("150405")
		}
	}

	// 特殊交易: GET PAN (60)
	if transType == "60" {
		if startGetPan == "" {
			ecrCode = "0001"
			hostResp = "05"
			diag.Reason = "缺少 Start Get PAN"
		} else if !isValidStartGetPan(startGetPan) {
			ecrCode = "0001"
			hostResp = "05"
			diag.Reason = "Start Get PAN 不合法"
		} else {
			setGetPan(&getPanState{
				Mode:   startGetPan,
				Amount: amountInt,
				Time:   now,
				CardNo: cardNo,
				HostID: hostID,
			})
		}
	}

	// 特殊交易: COMPLETE (62)
	if transType == "62" {
		st := getGetPan()
		if st == nil {
			ecrCode = "0001"
			hostResp = "05"
			diag.Reason = "缺少前置讀卡"
		} else if st.Amount > 0 && amountInt > st.Amount {
			ecrCode = "0001"
			hostResp = "05"
			diag.Reason = "金額超過前置讀卡"
		} else {
			cardNo = st.CardNo
		}
	}
	if isRefundTransType(transType) {
		hasRefInput := refReq != "" || invReq != "" || appReq != ""
		requiresRef := (transType == "27" && config.RequireRefForFisc) || (transType == "04" && config.RequireRefForInst) || config.RequireRefAndCard
		if requiresRef && !hasRefInput {
			ecrCode = "0001"
			hostResp = "05"
			diag.Reason = "缺少參照欄位"
		}
		if ecrCode == "0000" && config.RequireBatchOrCancel && batchReq == "" && cancelReq == "" {
			ecrCode = "0001"
			hostResp = "05"
			diag.Reason = "缺少批次/銷帳"
		}
		if ecrCode == "0000" && hasRefInput {
			if refReq != "" {
				matched = store.findByReference(refReq)
			} else if invReq != "" {
				matched = store.findByInvoice(invReq)
			} else if appReq != "" {
				matched = store.findByApproval(appReq)
			}
			if matched == nil {
				ecrCode = "0001"
				hostResp = "05"
				diag.Reason = "參照不存在"
			}
		}

		var cardToMatch string
		if ecrCode == "0000" && (config.RequireCardMatch || config.RequireRefAndCard) {
			if reqCard != "" {
				cardToMatch = reqCard
			} else if config.RefundCardMode == "random" {
				cardToMatch = store.randomCard()
			} else if last := store.latest(); last != nil {
				cardToMatch = last.CardNo
			}
			if cardToMatch == "" {
				ecrCode = "0001"
				hostResp = "05"
				if diag.Reason == "" {
					diag.Reason = "需刷原卡"
				}
			} else if matched == nil && config.RequireCardMatch {
				matched = store.findByCardAndAmount(cardToMatch, amountInt)
				if matched != nil {
					diag.MatchMode = "刷卡比對"
					diag.MatchCard = cardToMatch
				}
			}
		}

		if ecrCode == "0000" && matched == nil && config.RequireBatchOrCancel && (batchReq != "" || cancelReq != "") {
			matched = store.findByBatchOrCancel(batchReq, cancelReq)
			if matched != nil {
				diag.MatchMode = "批次/銷帳比對"
			}
		}

		if ecrCode == "0000" && matched != nil {
			if config.MaxRefundAgeHours > 0 {
				if int(time.Since(matched.Time).Hours()) > config.MaxRefundAgeHours {
					ecrCode = "0001"
					hostResp = "05"
					diag.Reason = "超過可退貨時效"
				}
			}
			if ecrCode == "0000" && transType == "27" {
				if matched.Amount > 0 && amountInt > matched.Amount {
					ecrCode = "0001"
					hostResp = "05"
					diag.Reason = "金額超過原交易"
				} else if matched.Reference != "" && store.isRefunded(matched.Reference) {
					ecrCode = "0001"
					hostResp = "05"
					diag.Reason = "原交易序號已退過"
				}
			}
			if ecrCode == "0000" && config.RequireRefAndCard {
				if cardToMatch == "" {
					ecrCode = "0001"
					hostResp = "05"
					if diag.Reason == "" {
						diag.Reason = "需刷原卡"
					}
				} else if matched.CardNo != cardToMatch {
					ecrCode = "0001"
					hostResp = "05"
					if diag.Reason == "" {
						diag.Reason = "卡號不一致"
					}
				} else {
					diag.MatchCard = cardToMatch
				}
			}
		}

		if ecrCode == "0000" && matched == nil {
			if config.MaxRefundAgeHours > 0 {
				ecrCode = "0001"
				hostResp = "05"
				diag.Reason = "無參照無法校驗時效"
			} else if hasRefInput || config.RequireCardMatch || config.RequireRefAndCard || config.RequireBatchOrCancel || requiresRef {
				ecrCode = "0001"
				hostResp = "05"
				if diag.Reason == "" {
					diag.Reason = "找不到可對應之原交易"
				}
			} else {
				diag.MatchMode = "無參照放行"
			}
		}

		if ecrCode == "0000" && matched != nil {
			if hasRefInput && diag.MatchMode == "" {
				diag.MatchMode = "參照欄位"
			}
			diag.MatchRef = matched.Reference
			diag.MatchInv = matched.InvoiceNo
			diag.MatchApp = matched.ApprovalNo
		}
	}

	if declined {
		ecrCode = "0001"
		hostResp = "05"
		diag.Reason = "模擬拒絕"
	}

	if ok {
		writeFieldByName(data, layout, "Trans_Type", transType)
		writeFieldByName(data, layout, "Host_ID", hostID)
		if amount != "" {
			writeFieldByName(data, layout, "Trans_Amount", amount)
		}
		writeFieldByName(data, layout, "Trans_Date", transDate)
		writeFieldByName(data, layout, "Trans_Time", transTime)
		writeFieldByName(data, layout, "EDC_Terminal_ID", terminalID)
		writeFieldByName(data, layout, "EDC_Merchant_ID", merchantID)
		if config.ReturnInvoice {
			writeFieldByName(data, layout, "Invoice_No", invoiceNo)
		}
		if config.ReturnReference {
			writeFieldByName(data, layout, "Reference_No", referenceNo)
		}
		if config.ReturnApproval {
			writeFieldByName(data, layout, "Approval_No", approvalNo)
		}
		if config.ReturnCard {
			writeFieldByName(data, layout, "Card_No", cardNo)
		}
		writeFieldByName(data, layout, "CardType", cardType)
		writeFieldByName(data, layout, "Card_Type", cardType)
		if config.RequireBatchOrCancel && transType != "27" {
			writeFieldByName(data, layout, "Batch_Number", batchNo)
			writeFieldByName(data, layout, "Cancel Debt Number", cancelDebt)
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

	// Record successful sale/inst-sale
	if ecrCode == "0000" && isSaleTransType(transType) {
		store.add(txRecord{
			TransType:  transType,
			InvoiceNo:  invoiceNo,
			Reference:  referenceNo,
			ApprovalNo: approvalNo,
			CardNo:     cardNo,
			BatchNo:    batchNo,
			CancelDebt: cancelDebt,
			Amount:     amountInt,
			Time:       now,
		})
	}
	if ecrCode == "0000" && transType == "62" {
		if st := getGetPan(); st != nil && (st.Mode == "01" || st.Mode == "03") {
			store.add(txRecord{
				TransType:  st.Mode,
				InvoiceNo:  invoiceNo,
				Reference:  referenceNo,
				ApprovalNo: approvalNo,
				CardNo:     cardNo,
				BatchNo:    batchNo,
				CancelDebt: cancelDebt,
				Amount:     amountInt,
				Time:       now,
			})
		}
	}

	frame := make([]byte, 0, FrameLen)
	frame = append(frame, STX)
	frame = append(frame, data...)
	frame = append(frame, ETX)
	lrc := calculateLRC(append(data, ETX))
	frame = append(frame, lrc)
	if ecrCode == "0000" && isRefundTransType(transType) && diag.MatchRef != "" {
		store.markRefunded(diag.MatchRef)
	}
	diag.ECR = ecrCode
	diag.Host = hostResp
	return frame, diag
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

func parseAmount(val string) int {
	val = strings.TrimSpace(val)
	if val == "" {
		return 0
	}
	n := 0
	for _, ch := range val {
		if ch < '0' || ch > '9' {
			continue
		}
		n = n*10 + int(ch-'0')
	}
	return n
}

func isRefundTransType(t string) bool {
	switch strings.TrimSpace(t) {
	case "02", "04", "24", "27", "32", "81":
		return true
	default:
		return false
	}
}

func isSaleTransType(t string) bool {
	switch strings.TrimSpace(t) {
	case "01", "03", "30", "80":
		return true
	default:
		return false
	}
}

func isValidStartGetPan(v string) bool {
	switch strings.TrimSpace(v) {
	case "01", "02", "03", "04":
		return true
	default:
		return false
	}
}

type diagInfo struct {
	TransType string
	Amount    string
	RefReq    string
	InvReq    string
	AppReq    string
	ReqCard   string
	BatchReq  string
	CancelReq string
	StartGet  string
	MatchMode string
	MatchCard string
	MatchRef  string
	MatchInv  string
	MatchApp  string
	ECR       string
	Host      string
	Reason    string
}

func logDecision(d diagInfo) {
	if !config.Verbose {
		return
	}
	lines := []string{
		"[MockPOS][交易判斷]",
		"  交易別: " + d.TransType,
		"  金額: " + d.Amount,
	}
	if d.RefReq != "" || d.InvReq != "" || d.AppReq != "" {
		lines = append(lines, "  參照欄位(請求): Reference="+d.RefReq+" Invoice="+d.InvReq+" Approval="+d.AppReq)
	}
	if d.StartGet != "" {
		lines = append(lines, "  Start Get PAN: "+d.StartGet)
	}
	if d.BatchReq != "" || d.CancelReq != "" {
		lines = append(lines, "  批次/銷帳(請求): Batch="+d.BatchReq+" CancelDebt="+d.CancelReq)
	}
	if d.ReqCard != "" {
		lines = append(lines, "  卡號(請求): "+d.ReqCard)
	}
	if d.MatchMode != "" {
		lines = append(lines, "  比對模式: "+d.MatchMode)
	}
	if d.MatchCard != "" {
		lines = append(lines, "  比對卡號: "+d.MatchCard)
	}
	if d.MatchRef != "" || d.MatchInv != "" || d.MatchApp != "" {
		lines = append(lines, "  比對原交易: Reference="+d.MatchRef+" Invoice="+d.MatchInv+" Approval="+d.MatchApp)
	}
	if d.Reason != "" {
		lines = append(lines, "  判斷原因: "+d.Reason)
	}
	lines = append(lines, "  回應碼: ECR="+d.ECR+" Host="+d.Host)
	for _, l := range lines {
		logVerbose(l)
	}
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
