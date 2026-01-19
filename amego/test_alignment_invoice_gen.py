#!/usr/bin/env python3
"""
Alignment Test for invoice_gen.py
This script outputs the intermediate values that TypeScript should match.
"""

import json
import hashlib

# --- Configuration (Must match invoice_gen.ts) ---
APP_KEY = "sHeq7t8G1wiQvhAuIM27"
INVOICE_TAX_ID = "12345678"

# --- Test Data (Must match reference.py) ---
INVOICE_DATA = {
    "OrderId": "A20200817106955",
    "BuyerIdentifier": "28080623",
    "BuyerName": "光貿科技有限公司",
    "NPOBAN": "",
    "ProductItem": [
        {
            "Description": "測試商品1",
            "Quantity": "1",
            "UnitPrice": "170",
            "Amount": "170",
            "Remark": "",
            "TaxType": "1"
        },
        {
            "Description": "會員折抵",
            "Quantity": "1",
            "UnitPrice": "-2",
            "Amount": "-2",
            "Remark": "",
            "TaxType": "1"
        }
    ],
    "SalesAmount": "160",
    "FreeTaxSalesAmount": "0",
    "ZeroTaxSalesAmount": "0",
    "TaxType": "1",
    "TaxRate": "0.05",
    "TaxAmount": "8",
    "TotalAmount": "168",
    "PrinterType": "2"
}

# Use a fixed timestamp for reproducible comparison
FIXED_TIMESTAMP = 1705420000


def generate_test_values():
    """Generate all intermediate values for comparison."""

    # 1. JSON Serialization with indent=0 (what reference.py uses)
    api_data_json_indent0 = json.dumps(INVOICE_DATA, indent=0)

    # 2. Compact JSON (for comparison)
    api_data_json_compact = json.dumps(INVOICE_DATA, separators=(',', ':'))

    # 3. Default JSON (for comparison)
    api_data_json_default = json.dumps(INVOICE_DATA)

    print("=" * 60)
    print("PYTHON OUTPUT - FOR TYPESCRIPT COMPARISON")
    print("=" * 60)

    print("\n--- 1. Invoice Data ---")
    print(f"OrderId: {INVOICE_DATA['OrderId']}")
    print(f"BuyerName: {INVOICE_DATA['BuyerName']}")
    print(f"TotalAmount: {INVOICE_DATA['TotalAmount']}")
    print(f"ProductItem count: {len(INVOICE_DATA['ProductItem'])}")

    print("\n--- 2. JSON Serialization Variants ---")
    print(f"\n[A] json.dumps(indent=0) (what Python code uses):")
    print(repr(api_data_json_indent0))
    print(f"Length: {len(api_data_json_indent0)}")
    print(f"\nFirst 200 chars:")
    print(repr(api_data_json_indent0[:200]))

    print(
        f"\n[B] json.dumps(separators=(',', ':')) (compact, like JS JSON.stringify):")
    print(f"Length: {len(api_data_json_compact)}")

    print(f"\n[C] json.dumps() default:")
    print(f"Length: {len(api_data_json_default)}")

    # 3. Signature Generation
    print("\n--- 3. Signature Generation (using fixed timestamp) ---")
    print(f"Fixed Timestamp: {FIXED_TIMESTAMP}")
    print(f"APP_KEY: {APP_KEY}")

    for label, json_str in [
        ("indent=0", api_data_json_indent0),
        ("compact", api_data_json_compact),
        ("default", api_data_json_default),
    ]:
        sig_source = f"{json_str}{FIXED_TIMESTAMP}{APP_KEY}"
        sig = hashlib.md5(sig_source.encode("utf-8")).hexdigest()
        print(f"\n[{label}]")
        print(f"  Signature Source Length: {len(sig_source)}")
        print(f"  MD5 Signature: {sig}")

    # 4. Form Data
    print("\n--- 4. Form Data Fields ---")
    print(f"invoice: {INVOICE_TAX_ID}")
    print(f"data: (JSON string from above)")
    print(f"time: {FIXED_TIMESTAMP}")
    print(f"sign: (MD5 signature from above)")

    print("\n" + "=" * 60)
    print("KEY VALUES FOR TYPESCRIPT COMPARISON:")
    print("=" * 60)

    # Calculate the signature with indent=0 (what Python uses)
    sig_source_indent0 = f"{api_data_json_indent0}{FIXED_TIMESTAMP}{APP_KEY}"
    sig_indent0 = hashlib.md5(sig_source_indent0.encode("utf-8")).hexdigest()

    print(f"""
JSON Length (indent=0): {len(api_data_json_indent0)}
MD5 Signature (indent=0): {sig_indent0}

TypeScript should match these values exactly!
""")

    print("=" * 60)
    print("RECOMMENDATION:")
    print("=" * 60)
    print("""
The Python code uses `json.dumps(invoice_data, indent=0)` which produces:
  - Newlines between key-value pairs
  - This is DIFFERENT from JavaScript's JSON.stringify()

TypeScript must implement jsonDumpsPythonIndent0() to match this behavior.
""")


if __name__ == "__main__":
    generate_test_values()
