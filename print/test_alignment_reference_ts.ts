/**
 * TypeScript Alignment Test for reference.ts
 * Run with: npx tsx print/test_alignment_reference_ts.ts
 *
 * This script outputs the same intermediate values as test_alignment_reference_py.py
 * so you can compare them side-by-side.
 */

import * as crypto from "crypto";

// --- Configuration (Must match Python) ---
const APP_KEY = "sHeq7t8G1wiQvhAuIM27";
const INVOICE_TAX_ID = "12345678";

// --- Test Data (Must match Python) ---
const INVOICE_DATA = {
  OrderId: "A20200817106955",
  BuyerIdentifier: "28080623",
  BuyerName: "光貿科技有限公司",
  NPOBAN: "",
  ProductItem: [
    {
      Description: "測試商品1",
      Quantity: "1",
      UnitPrice: "170",
      Amount: "170",
      Remark: "",
      TaxType: "1",
    },
    {
      Description: "會員折抵",
      Quantity: "1",
      UnitPrice: "-2",
      Amount: "-2",
      Remark: "",
      TaxType: "1",
    },
  ],
  SalesAmount: "160",
  FreeTaxSalesAmount: "0",
  ZeroTaxSalesAmount: "0",
  TaxType: "1",
  TaxRate: "0.05",
  TaxAmount: "8",
  TotalAmount: "168",
  PrinterType: "2",
};

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
      // Handle arrays (like ProductItem)
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
  // 1. JSON Serialization - Our Python-compatible version
  const apiDataJson = jsonDumpsPythonIndent0(INVOICE_DATA);

  // Also show what standard JSON.stringify produces for comparison
  const apiDataJsonStandard = JSON.stringify(INVOICE_DATA);

  console.log("=".repeat(60));
  console.log("TYPESCRIPT OUTPUT - FOR PYTHON COMPARISON");
  console.log("=".repeat(60));

  console.log("\n--- 1. Invoice Data ---");
  console.log(`OrderId: ${INVOICE_DATA.OrderId}`);
  console.log(`BuyerName: ${INVOICE_DATA.BuyerName}`);
  console.log(`TotalAmount: ${INVOICE_DATA.TotalAmount}`);
  console.log(`ProductItem count: ${INVOICE_DATA.ProductItem.length}`);

  console.log("\n--- 2. JSON Serialization ---");
  console.log(
    "\n[A] jsonDumpsPythonIndent0() (should match Python's indent=0):"
  );
  console.log(JSON.stringify(apiDataJson)); // Use JSON.stringify to show escape chars
  console.log(`Length: ${apiDataJson.length}`);
  console.log(`\nFirst 200 chars:`);
  console.log(JSON.stringify(apiDataJson.substring(0, 200)));

  console.log("\n[B] JSON.stringify() (standard JS - for reference):");
  console.log(`Length: ${apiDataJsonStandard.length}`);

  // 2. Signature Generation
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

  // 3. Run Python script first to get expected values
  console.log("\n" + "=".repeat(60));
  console.log("ALIGNMENT CHECK:");
  console.log("=".repeat(60));
  console.log(`
To verify alignment:
1. Run: python3 print/test_alignment_reference_py.py
2. Compare the output with this TypeScript output
3. The JSON Length and MD5 Signature should match exactly

TypeScript Results:
  JSON Length: ${apiDataJson.length}
  MD5 Signature: ${signature}
`);

  console.log("=".repeat(60));
  console.log("DETAILED JSON COMPARISON:");
  console.log("=".repeat(60));
  console.log("\nPython-compatible JSON (first 500 chars):");
  console.log(apiDataJson.substring(0, 500));
  console.log("\n...");
}

runTest();
