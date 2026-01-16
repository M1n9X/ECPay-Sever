/**
 * TypeScript Alignment Test
 * Run with: npx tsx print/test_alignment_ts.ts
 *
 * This script outputs the same intermediate values as test_alignment_py.py
 * so you can compare them side-by-side.
 */

import * as crypto from "crypto";

// --- Configuration (Must match Python) ---
const APP_KEY = "sHeq7t8G1wiQvhAuIM27";
const INVOICE_TAX_ID = "12345678";

// --- Test Parameters (Must match Python) ---
const TEST_ORDER_ID = "A20200817106955";
const TEST_PRINTER_TYPE = 2;
const FIXED_TIMESTAMP = 1705420000;

// --- Utility: MD5 ---
function md5(data: string): string {
  return crypto.createHash("md5").update(data).digest("hex");
}

// --- Utility: Match Python's json.dumps(indent=0) ---
function jsonDumpsPythonIndent0(obj: Record<string, any>): string {
  const entries = Object.entries(obj).map(([key, value]) => {
    const valueStr = typeof value === "string" ? `"${value}"` : String(value);
    return `"${key}": ${valueStr}`;
  });
  return "{\n" + entries.join(",\n") + "\n}";
}

// --- Main Test ---
function runTest() {
  // 1. Business Parameters
  const businessParams = {
    type: "order",
    order_id: TEST_ORDER_ID,
    printer_type: TEST_PRINTER_TYPE,
    print_invoice_type: 1,
  };

  // 2. JSON Serialization - Our Python-compatible version
  const apiDataJson = jsonDumpsPythonIndent0(businessParams);

  // Also show what standard JSON.stringify produces for comparison
  const apiDataJsonStandard = JSON.stringify(businessParams);

  console.log("=".repeat(60));
  console.log("TYPESCRIPT OUTPUT - FOR PYTHON COMPARISON");
  console.log("=".repeat(60));

  console.log("\n--- 1. Business Parameters ---");
  console.log(`order_id: ${TEST_ORDER_ID}`);
  console.log(`printer_type: ${TEST_PRINTER_TYPE}`);

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
  console.log(`  Signature Source: ${JSON.stringify(sigSource)}`);
  console.log(`  MD5 Signature: ${signature}`);

  // Compare with standard
  const sigSourceStd = `${apiDataJsonStandard}${FIXED_TIMESTAMP}${APP_KEY}`;
  const signatureStd = md5(sigSourceStd);
  console.log("\n[Standard JSON.stringify - for reference]:");
  console.log(`  MD5 Signature: ${signatureStd}`);

  // 4. Expected Python Value
  console.log("\n" + "=".repeat(60));
  console.log("EXPECTED PYTHON VALUES (from test_alignment_py.py):");
  console.log("=".repeat(60));
  console.log(`
[indent=0]
  JSON Length: 94
  MD5 Signature: 1128938d94c522ac4ee962fdbd063687
`);

  // 5. Final Verdict
  console.log("=".repeat(60));
  console.log("ALIGNMENT CHECK:");
  console.log("=".repeat(60));

  const pythonSig = "1128938d94c522ac4ee962fdbd063687";
  if (signature === pythonSig && apiDataJson.length === 94) {
    console.log("✅ SUCCESS! TypeScript output matches Python exactly.");
  } else {
    console.log("❌ MISMATCH DETECTED!");
    console.log(`  Expected Length: 94, Got: ${apiDataJson.length}`);
    console.log(`  Expected Sig: ${pythonSig}`);
    console.log(`  Got Sig:      ${signature}`);
  }
}

runTest();
