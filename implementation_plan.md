# ECPay POS Integration Implementation Plan

## Goal

Implement a complete solution for ECPay POS integration including a Go Server (acting as RS232 Bridge) and a modern Web App for user interaction. The system must support Payment (Sale), Refund, and other standard operations.

## Architecture

- **Web App**: React + Vite application. Modern UI. Connects to Server via WebSocket.
- **Go Server**: Middleware. Manages RS232 connection to POS. Exposes WebSocket API to Web App.
- **Protocol**: ECPay RS232 Protocol (STX, ETX, LRC, SHA1).
- **Communication Flow**: `Web UI -> JSON/WebSocket -> Go Server -> RS232/Packets -> POS`

## User Review Required
>
> [!IMPORTANT]
> **Hardware Simulation**: Since physical POS hardware is not available for testing, I will implement a **Mock Serial Mode** in the Go Server. This mode will simulate POS responses (ACKs, delayed Transaction Results) to verify the full software stack.

## Proposed Changes

### 1. Go Server (`server/`)

Structure based on the Design Doc:

```
server/
├── main.go             # Entry point
├── config/
│   └── config.go       # Configuration (Port, BaudRate, MockMode)
├── driver/
│   ├── serial.go       # Real Serial Port Implementation
│   ├── mock.go         # Mock Serial Port Implementation
│   └── manager.go      # Serial Manager (handles read/write loop)
├── protocol/
│   ├── packet.go       # ECPay Packet Structs
│   ├── crypto.go       # LRC and SHA1 Logic
│   └── parser.go       # Packet Parser & Validator
└── api/
    └── websocket.go    # WebSocket Server & Handler
```

### 2. Web App (`webapp/`)

New Vite React Project:

- **Tech Stack**: React, Vite, TailwindCSS (for styling), Lucide React (Icons).
- **Features**:
  - **Dashboard**: Connection Status (Server & POS).
  - **Payment Tab**: A keypad or amount input to trigger Sale.
  - **Refund Tab**: Input for OrderNo and Amount.
  - **Transaction Log**: Display recent transactions.
- **Styling**: "Rich Aesthetics" as requested - Dark mode, gradients, smooth animations.

## Verification Plan

### Automated Tests

- **Protocol Unit Tests**: Verify `CalculateLRC` and `GenerateCheckMacValue` (SHA1) against known test vectors from docs.
- **Packet Building Tests**: Verify `BuildSalePacket` produces correct byte sequences.

### Manual Verification (Walkthrough)

1. **Start Server in Mock Mode**: `go run main.go --mock`
2. **Start Web App**: `npm run dev`
3. **Test Connection**: Web App should show "Connected".
4. **Test Sale**:
    - Enter Amount 100 on Web.
    - Click "Checkout".
    - UI shows "Processing... Please swipe card".
    - (Mock Server simulates Wait -> Response).
    - UI shows "Success" with Mock Auth Code.
5. **Test Refund**:
    - Go to Refund Tab.
    - Enter OrderNo and Amount.
    - Click "Refund".
    - Verify Success flow.
6. **Test Error Handling**:
    - Trigger a "Timeout" scenario (if adjustable in Mock).

## Directory Structure

Root: `/Users/mxue/GitRepos/FlowDance/ECPay-Sever`

- `server/`
- `webapp/`
- `docs/` (Existing)
