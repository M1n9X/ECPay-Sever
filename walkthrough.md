# ECPay POS Integration Walkthrough

This document outlines the implemented solution for the ECPay POS RS232 integration, including how to run the components and verify functionality.

## System Components

1. **Go Server (`server/`)**: Acts as a middleware bridge.
    - **Mode 1: Real**: Connects to physical RS232 port (e.g., COM3).
    - **Mode 2: Mock**: Simulates a POS device behavior for testing without hardware.
    - **API**: Exposes a WebSocket server at `ws://localhost:8989/ws`.
2. **Web App (`webapp/`)**: A modern React application using Vite and TailwindCSS.
    - Connects to the Go Server.
    - Provides UI for Payment (Sale) and Refund.

## Prerequisites

- **Go 1.20+**
- **Node.js 18+**

## How to Run

### 1. Start the Go Server

You can run the server in Mock mode for testing, or Real mode for actual hardware connection.

**Mock Mode (Simulation):**

```bash
cd server
go build -o ecpay-server
./ecpay-server --mock
```

**Real Mode (Hardware):**

```bash
cd server
# Replace COM3 with your actual serial port
./ecpay-server --port COM3
```

*Note: Ensure you have the correct drivers installed for your USB-to-Serial adapter.*

### 2. Start the Web App

In a separate terminal:

```bash
cd webapp
npm run dev
```

Access the app at: <http://localhost:5173>

## Functionality & Verification

### Payment (Sale) Flow

1. Open the Web App. Ensure the status indicator says **"POS Online"**.
2. In the **Sale** tab, enter an amount (e.g., `100` for $1.00).
3. Click **Charge**.
4. The system will show **"Processing"** (waiting for POS response).
5. In Mock mode, it will auto-approve after 2 seconds. In Real mode, swipe the card on the POS.
6. The UI shows **"Approved"** with the Authorization Code.

### Refund Flow

1. Switch to the **Refund** tab.
2. Enter the **Original Order No** (20 digits).
3. Enter the refund **Amount**.
4. Click **Refund**.
5. System processes and shows approval.

## Implementation Details

- **Protocol Implementation**: Full implementation of ECPay RS232 specs, including STX/ETX framing, LRC validation, and SHA1 hash generation in `server/protocol`.
- **Driver Architecture**: Clean separation between `SerialManager` (business logic) and `Port` (hardware interface), enabling easy mocking.
- **Security**: SHA1 hashing prevents tampering. Input validation ensures safe serial communication.
- **UI**: "Rich Aesthetics" using dark mode, gradients, and animations.

## Files Created

- `server/protocol/*.go`: Core protocol logic.
- `server/driver/*.go`: Serial port management.
- `webapp/src/hooks/usePOS.ts`: WebSocket logic.
- `webapp/src/App.tsx`: Main UI.

---
**Verification Recording**:
![ECPay Verification Demo](/Users/mxue/.gemini/antigravity/brain/97940ddd-7e94-4c44-a224-7b3c6957956f/ecpay_final_verification_1768495395301.webp)
