package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"time"
)

const (
	STX       byte = 0x02
	ETX       byte = 0x03
	ACK       byte = 0x06
	NAK       byte = 0x15
	PacketLen      = 600
)

func main() {
	listener, err := net.Listen("tcp", ":9999")
	if err != nil {
		fmt.Println("Failed to start Mock POS:", err)
		return
	}
	defer listener.Close()

	fmt.Println("=== Mock POS Simulator ===")
	fmt.Println("Listening on TCP :9999")
	fmt.Println("Waiting for connections...")

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Accept error:", err)
			continue
		}
		fmt.Println("[MockPOS] Client connected:", conn.RemoteAddr())
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	buf := make([]byte, 1024)
	var accumulator []byte

	for {
		n, err := conn.Read(buf)
		if err != nil {
			fmt.Println("[MockPOS] Connection closed")
			return
		}

		accumulator = append(accumulator, buf[:n]...)

		// Try to parse a complete packet
		if len(accumulator) >= 603 {
			// Find STX
			stxIdx := bytes.IndexByte(accumulator, STX)
			if stxIdx >= 0 && len(accumulator) >= stxIdx+603 {
				packet := accumulator[stxIdx : stxIdx+603]
				accumulator = accumulator[stxIdx+603:]

				// Validate packet
				if validatePacket(packet) {
					fmt.Println("[MockPOS] Valid packet received. Sending ACK...")
					conn.Write([]byte{ACK})

					// Simulate processing delay
					fmt.Println("[MockPOS] Processing transaction (2s delay)...")
					time.Sleep(2 * time.Second)

					// Build and send response
					response := buildResponse(packet)
					conn.Write(response)
					fmt.Println("[MockPOS] Response sent.")
				} else {
					fmt.Println("[MockPOS] Invalid packet. Sending NAK.")
					conn.Write([]byte{NAK})
				}
			}
		}
	}
}

func validatePacket(packet []byte) bool {
	if len(packet) != 603 {
		return false
	}
	if packet[0] != STX || packet[601] != ETX {
		return false
	}

	// Validate LRC
	payload := packet[1:602]
	recLrc := packet[602]
	calcLrc := calculateLRC(payload)
	return calcLrc == recLrc
}

func calculateLRC(data []byte) byte {
	var lrc byte = 0
	for _, b := range data {
		lrc ^= b
	}
	return lrc
}

func buildResponse(reqPacket []byte) []byte {
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
	copy(data[4:10], []byte(fmt.Sprintf("%06d", now.Unix()%1000000)))

	// Mock Card Number (Offset 10, Len 19) - masked
	copy(data[10:29], []byte("4311****1234       "))

	// Trans Date (Offset 43, Len 6) - YYMMDD
	copy(data[43:49], []byte(now.Format("060102")))

	// Trans Time (Offset 49, Len 6) - hhmmss
	copy(data[49:55], []byte(now.Format("150405")))

	// Approval Number (Offset 55, Len 6)
	copy(data[55:61], []byte(fmt.Sprintf("%06d", now.Unix()%1000000)))

	// Response Code (Offset 61, Len 4) - SUCCESS
	copy(data[61:65], []byte("0000"))

	// Terminal ID (Offset 65, Len 8)
	copy(data[65:73], []byte("TERM0001"))

	// Merchant ID (Offset 73, Len 15)
	copy(data[73:88], []byte("MER000123456789"))

	// EC Order Number (Offset 88, Len 20)
	orderNo := fmt.Sprintf("MOCK%s", now.Format("20060102150405"))
	copy(data[88:108], []byte(fmt.Sprintf("%-20s", orderNo)))

	// Card Type (Offset 126, Len 2) - VISA
	copy(data[126:128], []byte("00"))

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

	// Build frame
	frame := new(bytes.Buffer)
	frame.WriteByte(STX)
	frame.Write(data)
	frame.WriteByte(ETX)

	lrcPayload := append(data, ETX)
	lrc := calculateLRC(lrcPayload)
	frame.WriteByte(lrc)

	return frame.Bytes()
}
