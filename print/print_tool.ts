import * as http from "http";
import * as crypto from "crypto";
import * as url from "url";

// --- Configuration ---
const HOST = "localhost";
const PORT = 3000;
const EXTERNAL_API_URL = "https://invoice-api.amego.tw/json/invoice_print";
const APP_KEY = "sHeq7t8G1wiQvhAuIM27";
const INVOICE_TAX_ID = "12345678";

// --- Utility: MD5 Hash ---
function md5(data: string): string {
  return crypto.createHash("md5").update(data).digest("hex");
}

// --- Utility: Match Python's json.dumps(indent=0) ---
// Python's json.dumps(obj, indent=0) produces output like:
// '{\n"key": "value",\n"key2": 123\n}'
// Note: newlines between entries, space after colon, no trailing comma
function jsonDumpsPythonIndent0(obj: Record<string, any>): string {
  const entries = Object.entries(obj).map(([key, value]) => {
    const valueStr = typeof value === "string" ? `"${value}"` : String(value);
    return `"${key}": ${valueStr}`;
  });
  return "{\n" + entries.join(",\n") + "\n}";
}

// --- HTML UI Content ---
// We embed this directly so the file is standalone.
const HTML_CONTENT = `
<!DOCTYPE html>
<html>
<head>
    <meta http-equiv="Content-Type" content="text/html; charset=utf-8" />
    <title>Invoice Print Tool</title>
    <style>
        body { font-family: -apple-system, system-ui, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", sans-serif; padding: 2rem; max-width: 800px; margin: 0 auto; background-color: #f9f9fa; color: #333; }
        .card { background: white; padding: 2rem; border-radius: 12px; box-shadow: 0 4px 6px rgba(0,0,0,0.05); border: 1px solid #e1e4e8; }
        h1 { margin-top: 0; font-size: 1.5rem; color: #24292e; border-bottom: 2px solid #f1f1f1; padding-bottom: 1rem; }
        
        .grid { display: grid; grid-template-columns: 1fr 1fr; gap: 1.5rem; margin-bottom: 1.5rem; }
        .full { grid-column: span 2; }
        
        .form-group { margin-bottom: 0.5rem; }
        label { display: block; font-weight: 600; font-size: 0.9rem; margin-bottom: 0.5rem; color: #586069; }
        input { width: 100%; padding: 0.75rem; border: 1px solid #d1d5da; border-radius: 6px; font-size: 1rem; box-sizing: border-box; transition: border-color 0.2s; }
        input:focus { border-color: #0366d6; outline: none; box-shadow: 0 0 0 3px rgba(3,102,214,0.1); }
        
        .actions { display: flex; gap: 1rem; margin-top: 2rem; }
        button { flex: 1; padding: 0.75rem; font-size: 1rem; font-weight: 600; border: none; border-radius: 6px; cursor: pointer; transition: all 0.2s; }
        button:disabled { opacity: 0.6; cursor: not-allowed; }
        
        .btn-usb { background-color: #6c757d; color: white; }
        .btn-usb:hover:not(:disabled) { background-color: #5a6268; }
        
        .btn-print { background-color: #28a745; color: white; flex: 2; }
        .btn-print:hover:not(:disabled) { background-color: #218838; }
        
        #status-area { margin-top: 2rem; padding: 1rem; border-radius: 6px; background: #fafbfc; border: 1px solid #e1e4e8; font-family: monospace; font-size: 0.9rem; white-space: pre-wrap; min-height: 60px; }
        .status-log { margin-bottom: 0.5rem; border-bottom: 1px dashed #eee; padding-bottom: 0.5rem; }
        .status-log:last-child { border-bottom: none; }
        .log-time { color: #999; margin-right: 0.5rem; }
        .log-error { color: #d73a49; }
        .log-success { color: #22863a; }
        .log-info { color: #005cc5; }
    </style>
</head>
<body>

<div class="card">
    <h1>🖨️ Invoice Print Tool</h1>

    <div class="grid">
        <div class="form-group full">
            <label for="orderId">Order ID (訂單編號)</label>
            <input type="text" id="orderId" value="A20200817106955" placeholder="e.g. A20200817106955">
        </div>
        <div class="form-group">
            <label for="printerType">Printer Type</label>
            <input type="number" id="printerType" value="2">
        </div>
        <div class="form-group">
             <label for="taxId">Tax ID (統編)</label>
             <input type="text" id="taxId" value="${INVOICE_TAX_ID}" readonly style="background:#eee; cursor:not-allowed">
        </div>
    </div>

    <div class="actions">
        <button id="btnPair" class="btn-usb">🔗 1. Pair Printer</button>
        <button id="btnPrint" class="btn-print" disabled>🖨️ 2. Fetch & Print</button>
    </div>

    <div id="status-area">Waiting for user action...</div>
</div>

<script>
    // --- Logging ---
    function log(msg, type = 'info') {
        const area = document.getElementById('status-area');
        const line = document.createElement('div');
        line.className = 'status-log log-' + type;
        const time = new Date().toLocaleTimeString();
        line.innerHTML = '<span class="log-time">[' + time + ']</span> ' + msg;
        area.insertBefore(line, area.firstChild);
    }

    // --- WebUSB Logic ---
    let usbDevice = null;
    const ENDPOINT_NUMBER = 1;

    // Check Support
    if (!navigator.usb) {
        log("❌ WebUSB is not supported in this browser. Please use Chrome or Edge.", "error");
        document.getElementById('btnPair').disabled = true;
    } else {
        // Auto Connect
        navigator.usb.getDevices().then(devices => {
            if (devices.length > 0) {
                usbDevice = devices[0];
                log("✅ Found paired device: " + usbDevice.productName, "success");
                connectDevice(false);
            }
        });
    }

    document.getElementById('btnPair').onclick = async () => {
        try {
            usbDevice = await navigator.usb.requestDevice({
                filters: [
                    { vendorId: 0x0483 }, // Xprinter
                    { vendorId: 0x04D8, productId: 0x000A } // WP-T810
                ]
            });
            log("✅ Paired with: " + usbDevice.productName, "success");
            await connectDevice();
        } catch (e) {
            log("❌ Pairing failed: " + e.message, "error");
        }
    };

    async function connectDevice(showAlert = true) {
        if (!usbDevice) return;
        
        try {
            if (!usbDevice.opened) {
                await usbDevice.open();
                await usbDevice.selectConfiguration(1);
                
                // Find Out Endpoint
                let foundInterface = false;
                for (const iface of usbDevice.configuration.interfaces) {
                    for (const ep of iface.alternate.endpoints) {
                        if (ep.direction === 'out') {
                            await usbDevice.claimInterface(iface.interfaceNumber);
                            foundInterface = true;
                            break;
                        }
                    }
                    if (foundInterface) break;
                }
                
                if (!foundInterface) throw new Error("No suitable interface found");
            }
            
            document.getElementById('btnPrint').disabled = false;
            log("✅ Printer Connected Ready", "success");
            if(showAlert) alert("Printer Connected!");
        } catch (e) {
            usbDevice = null;
            document.getElementById('btnPrint').disabled = true;
            log("❌ Connection Error: " + e.message, "error");
        }
    }

    function base64ToUint8Array(base64) {
        const binaryString = window.atob(base64);
        const len = binaryString.length;
        const bytes = new Uint8Array(len);
        for (let i = 0; i < len; i++) {
            // Match original HTML: handle negative values (defensive)
            let v = binaryString.charCodeAt(i);
            v = v < 0 ? (v + 256) : v;
            bytes[i] = v;
        }
        return bytes;
    }

    async function sendToPrinter(base64Data) {
        if (!usbDevice || !usbDevice.opened) {
            throw new Error("Printer not connected");
        }
        
        const data = base64ToUint8Array(base64Data);
        // Match original HTML: global endpoint, retry up to 15
        let ep = 1; 
        
        while (ep <= 15) {
            try {
                await usbDevice.transferOut(ep, data);
                return; // Success
            } catch (e) {
                if (e.message.includes('endpoint') && ep < 15) {
                    ep++;
                    log("⚠️ Retrying with endpoint " + ep, "info");
                } else {
                    throw e;
                }
            }
        }
    }

    // --- Main Logic ---
    document.getElementById('btnPrint').onclick = async () => {
        const btn = document.getElementById('btnPrint');
        const orderId = document.getElementById('orderId').value;
        const printerType = document.getElementById('printerType').value;
        
        if(!orderId) {
            alert("Please enter Order ID");
            return;
        }

        btn.disabled = true;
        log("🔄 Fetching invoice data...", "info");

        try {
            // 1. Call Local Proxy
            const resp = await fetch('/api/proxy', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    order_id: orderId,
                    printer_type: parseInt(printerType)
                })
            });
            
            if(!resp.ok) throw new Error("Proxy Server Error: " + resp.statusText);
            
            const result = await resp.json();
            if(!result.success) throw new Error(result.error || "Unknown API Error");
            
            log("✅ Data received (" + result.base64.length + " bytes)", "success");
            
            // 2. Print
            log("🖨️ Sending to printer...", "info");
            await sendToPrinter(result.base64);
            
            log("✅ PRINT SUCCESSFUL", "success");
            alert("Success!");
            
        } catch (e) {
            log("❌ Error: " + e.message, "error");
            console.error(e);
        } finally {
            btn.disabled = false;
        }
    };
</script>
</body>
</html>
`;

// --- Server Logic ---

const server = http.createServer(async (req: any, res: any) => {
  // CORS Headers (for safety, though likely same-origin)
  res.setHeader("Access-Control-Allow-Origin", "*");
  res.setHeader("Access-Control-Allow-Methods", "GET, POST, OPTIONS");
  res.setHeader("Access-Control-Allow-Headers", "Content-Type");

  if (req.method === "OPTIONS") {
    res.writeHead(204);
    res.end();
    return;
  }

  const reqUrl = url.parse(req.url || "", true);

  // 1. Serve UI
  if (
    req.method === "GET" &&
    (reqUrl.pathname === "/" || reqUrl.pathname === "/index.html")
  ) {
    res.writeHead(200, { "Content-Type": "text/html" });
    res.end(HTML_CONTENT);
    return;
  }

  // 2. Proxy API
  if (req.method === "POST" && reqUrl.pathname === "/api/proxy") {
    let body = "";
    req.on("data", (chunk: any) => (body += chunk.toString()));
    req.on("end", async () => {
      try {
        const clientData = JSON.parse(body);
        const orderId = clientData.order_id;
        const printerType = clientData.printer_type || 2;

        console.log(`[Proxy] Fetching order: ${orderId}, type: ${printerType}`);

        // --- Business Logic from Python ---
        // 1. Prepare Params
        const businessParams = {
          type: "order",
          order_id: orderId,
          printer_type: printerType,
          print_invoice_type: 1,
        };

        // 2. Serialize to match Python's json.dumps(indent=0) EXACTLY
        // Python's indent=0 produces: '{\n"key": "value",\n...}'
        // We must replicate this for signature compatibility
        const apiDataJson = jsonDumpsPythonIndent0(businessParams);

        // 3. Sign
        const timestamp = Math.floor(Date.now() / 1000);
        const signatureSource = `${apiDataJson}${timestamp}${APP_KEY}`;
        const signature = md5(signatureSource);

        // 4. Send to External API
        // Using standard form-urlencoded format
        const postData = new URLSearchParams();
        postData.append("invoice", INVOICE_TAX_ID);
        postData.append("data", apiDataJson);
        postData.append("time", timestamp.toString());
        postData.append("sign", signature);

        const postDataStr = postData.toString();

        const apiReq = await fetch(EXTERNAL_API_URL, {
          method: "POST",
          headers: {
            "Content-Type": "application/x-www-form-urlencoded",
          },
          body: postDataStr,
        });

        if (!apiReq.ok) {
          throw new Error(`External API returned ${apiReq.status}`);
        }

        const apiResp: any = await apiReq.json();

        // Validate Response
        if (apiResp.code === 0 && apiResp.data && apiResp.data.base64_data) {
          res.writeHead(200, { "Content-Type": "application/json" });
          res.end(
            JSON.stringify({
              success: true,
              base64: apiResp.data.base64_data,
            })
          );
        } else {
          res.writeHead(200, { "Content-Type": "application/json" });
          res.end(
            JSON.stringify({
              success: false,
              error: apiResp.msg || "Unknown API Error",
              raw: apiResp,
            })
          );
        }
      } catch (err: any) {
        console.error("Proxy Error:", err);
        res.writeHead(500, { "Content-Type": "application/json" });
        res.end(JSON.stringify({ success: false, error: err.message }));
      }
    });
    return;
  }

  // 404
  res.writeHead(404);
  res.end("Not Found");
});

// --- Start ---
server.listen(PORT, HOST, () => {
  console.log(`
🚀 Print Tool Running!
----------------------------
👉 Open: http://${HOST}:${PORT}
----------------------------
Stop server with Ctrl+C
`);
});
