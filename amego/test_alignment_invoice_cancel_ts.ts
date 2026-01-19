/**
 * TypeScript Alignment Test for invoice_cancel.ts
 * Run with: npx tsx print/test_alignment_invoice_cancel_ts.ts
 *
 * This script outputs the same intermediate values as test_alignment_invoice_cancel_py.py
 * so you can compare them side-by-side.
 */

import * as crypto from "crypto";

// --- Configuration (Must match Python) ---
const APP_KEY = "sHeq7t8G1wiQvhAuIM27";
const INVOICE_TAX_ID = "12345678";

// --- Test Data ---
const TEST_INVOICE_NUMBER = "AB12345678";
const TEST_CANCEL_REASON = "測試作廢";

const FIXED_TIMESTAMP = 1705420000;

// --- Utility: MD5 ---
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
function jsonDumpsPythonIndent0(obj: Record<string, any>): string {
  const entries: string[] = [];

  for (const [key, value] of Object.entries(obj)) {
    let valueStr: string;

    if (typeof value === "string") {
      valueStr = `"${escapeStringPython(value)}"`;
    } else if (Array.isArray(value)) {
      // Handle arrays
      const arrayItems = value.map((item) => {
        if (typeof item === "object" && item !== null) {
          return jsonDumpsPythonIndent0(item);
        }
        return typeof item === "string" ? `"${escapeStringPython(item)}"` : String(item);
      });
      valueStr = "[\n" + arrayItems.join(",\n") + "\n]";
    } else if (typeof value === "object" && value !== null) {
      valueStr = jsonDumpsPythonIndent0(value);
    } else {
      valueStr = String(value);
    }

    entries.push(`"${key}": ${valueStr}`);
  }

  return "{\n" + entries.join(",\n") + "\n}";
}

// --- Main Test ---
function runTest() {
  // 1. Business Parameters
  const businessParams = {
    InvoiceNumber: TEST_INVOICE_NUMBER,
    Reason: TEST_CANCEL_REASON,
  };

  // 2. JSON Serialization - Our Python-compatible version
  const apiDataJson = jsonDumpsPythonIndent0(businessParams);

  // Also show what standard JSON.stringify produces for comparison
  const apiDataJsonStandard = JSON.stringify(businessParams);

  console.log("=".repeat(60));
  console.log("TYPESCRIPT OUTPUT - FOR PYTHON COMPARISON");
  console.log("=".repeat(60));

  console.log("\n--- 1. Business Parameters ---");
  console.log(`InvoiceNumber: ${TEST_INVOICE_NUMBER}`);
  console.log(`Reason: ${TEST_CANCEL_REASON}`);

  console.log("\n--- 2. JSON Serialization ---");
  console.log(
    "\n[A] jsonDumpsPythonIndent0() (should match Python's indent=0):"
  );
  console.log(JSON.stringify(apiDataJson)); // Use JSON.stringify to show escape chars
  console.log(`Length: ${apiDataJson.length}`);

  console.log("\n[B] JSON.stringify() (standard JS - for reference):");
  console.log(JSON.stringify(apiDataJsonStandard));
  console.log(`Length: ${apiDataJsonStandard.length}`);

  // 3. Signature Generation
  console.log("\n--- 3. Signature Generation (using fixed timestamp) ---");
  console.log(`Fixed Timestamp: ${FIXED_TIMESTAMP}`);
  console.log(`APP_KEY: ${APP_KEY}`);

  const sigSource = `${apiDataJson}${FIXED_TIMESTAMP}${APP_KEY}`;
  const signature = md5(sigSource);

  console.log("\n[Python-compatible (indent=0)]:");
  console.log(`  Signature Source Length: ${sigSource.length}`);
  console.log(`  MD5 Signature: ${signature}`);

  // Compare with standard
  const sigSourceStd = `${apiDataJsonStandard}${FIXED_TIMESTAMP}${APP_KEY}`;
  const signatureStd = md5(sigSourceStd);
  console.log("\n[Standard JSON.stringify - for reference]:");
  console.log(`  MD5 Signature: ${signatureStd}`);

  // 4. Final Verdict
  console.log("\n" + "=".repeat(60));
  console.log("ALIGNMENT CHECK:");
  console.log("=".repeat(60));
  console.log(`
To verify alignment:
1. Run: python3 test_alignment_invoice_cancel_py.py
2. Compare the output with this TypeScript output
3. The JSON Length and MD5 Signature should match exactly

TypeScript Results:
  JSON Length: ${apiDataJson.length}
  MD5 Signature: ${signature}
`);
}

runTest();
