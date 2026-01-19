/**
 * TypeScript version of invoice_cancel.py
 * Invoice Cancellation (作廢發票)
 *
 * This script demonstrates cancelling an invoice using the Amego Invoice API.
 * It matches the Python invoice_cancel implementation exactly.
 *
 * Run with: npx tsx print/invoice_cancel.ts
 */

import * as crypto from "crypto";

// --- Configuration ---
const API_URL = "https://invoice-api.amego.tw/json/f0501";
const APP_KEY = "sHeq7t8G1wiQvhAuIM27";
const INVOICE_TAX_ID = "12345678"; // 統編

// --- Utility: MD5 Hash ---
function md5(data: string): string {
  return crypto.createHash("md5").update(data).digest("hex");
}

// --- Utility: Escape string to match Python's json.dumps (ASCII-only) ---
function escapeStringPython(str: string): string {
  let result = "";
  for (let i = 0; i < str.length; i++) {
    const char = str[i];
    const code = str.charCodeAt(i);

    // Escape special characters
    if (char === '"') result += '\\"';
    else if (char === "\\") result += "\\\\";
    else if (char === "\n") result += "\\n";
    else if (char === "\r") result += "\\r";
    else if (char === "\t") result += "\\t";
    // Python's json.dumps escapes non-ASCII characters to \uXXXX
    else if (code > 127) {
      result += "\\u" + code.toString(16).padStart(4, "0");
    } else {
      result += char;
    }
  }
  return result;
}

// --- Utility: Match Python's json.dumps(indent=0) ---
// Python's json.dumps(obj, indent=0) produces output like:
// '{\n"key": "value",\n"key2": 123\n}'
// Note: newlines between entries, space after colon, no trailing comma
// Also escapes non-ASCII characters to \uXXXX format
function jsonDumpsPythonIndent0(obj: any): string {
  if (Array.isArray(obj)) {
    const arrayItems = obj.map((item) => {
      if (typeof item === "object" && item !== null) {
        return jsonDumpsPythonIndent0(item);
      }
      return typeof item === "string"
        ? `"${escapeStringPython(item)}"`
        : String(item);
    });
    return "[\n" + arrayItems.join(",\n") + "\n]";
  }

  const entries: string[] = [];

  for (const [key, value] of Object.entries(obj)) {
    let valueStr: string;

    if (typeof value === "string") {
      valueStr = `"${escapeStringPython(value)}"`;
    } else if (typeof value === "object" && value !== null) {
      valueStr = jsonDumpsPythonIndent0(value);
    } else {
      valueStr = String(value);
    }

    entries.push(`"${key}": ${valueStr}`);
  }

  return "{\n" + entries.join(",\n") + "\n}";
}

// --- Main Function ---
async function cancelInvoice(invoiceNumber: string): Promise<any> {
  // Business Parameters
  // API expects an Array of objects with CancelInvoiceNumber
  const businessParams = [
    {
      CancelInvoiceNumber: invoiceNumber,
    },
  ];

  // Convert to JSON matching Python's json.dumps(indent=0)
  const apiDataJson = jsonDumpsPythonIndent0(businessParams);

  // Unix Timestamp (10 digits, no milliseconds)
  const currentTimestamp = Math.floor(Date.now() / 1000);

  // Generate MD5 signature
  const hashText = `${apiDataJson}${currentTimestamp}${APP_KEY}`;
  const signature = md5(hashText);

  console.log("=".repeat(60));
  console.log("Invoice Cancellation (作廢發票)");
  console.log("=".repeat(60));
  console.log(`\nAPI URL: ${API_URL}`);
  console.log(`Invoice Number: ${invoiceNumber}`);
  console.log(`Timestamp: ${currentTimestamp}`);
  console.log(`Signature: ${signature}`);

  // Prepare POST data
  const postData = new URLSearchParams();
  postData.append("invoice", INVOICE_TAX_ID);
  postData.append("data", apiDataJson);
  postData.append("time", currentTimestamp.toString());
  postData.append("sign", signature);

  try {
    console.log("\n" + "=".repeat(60));
    console.log("Sending request to API...");
    console.log("=".repeat(60));

    const response = await fetch(API_URL, {
      method: "POST",
      headers: {
        "Content-Type": "application/x-www-form-urlencoded",
      },
      body: postData.toString(),
    });

    if (!response.ok) {
      throw new Error(`HTTP Error: ${response.status} ${response.statusText}`);
    }

    const result = await response.json();

    console.log("\n" + "=".repeat(60));
    console.log("API Response:");
    console.log("=".repeat(60));
    console.log(JSON.stringify(result, null, 2));

    if (result.code === 0) {
      console.log(
        `\n✅ SUCCESS! Invoice ${invoiceNumber} cancelled successfully.`,
      );
    } else {
      console.log(`\n❌ ERROR: ${result.msg || "Unknown error"}`);
    }

    return result;
  } catch (error: any) {
    console.error("\n❌ Request failed:", error.message);
    return null;
  }
}

// --- Run ---
if (require.main === module) {
  const TEST_INVOICE_NUMBER = "AB12345678";
  cancelInvoice(TEST_INVOICE_NUMBER);
}

export { jsonDumpsPythonIndent0, md5, cancelInvoice };
