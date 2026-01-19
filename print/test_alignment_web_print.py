#!/usr/bin/env python3
"""
Comparison Test: Verifies that the TypeScript logic matches the Python logic.
This script outputs the intermediate values that TypeScript should match.
"""

import json
import hashlib

# --- Configuration (Must match web_print_tool.ts) ---
APP_KEY = "sHeq7t8G1wiQvhAuIM27"
INVOICE_TAX_ID = "12345678"

# --- Test Parameters ---
TEST_ORDER_ID = "A20200817106955"
TEST_PRINTER_TYPE = 2
# Use a fixed timestamp for reproducible comparison
FIXED_TIMESTAMP = 1705420000  # Example fixed timestamp


def generate_test_values():
    """Generate all intermediate values for comparison."""

    # 1. Business Parameters (must match exactly)
    business_params = {
        "type": "order",
        "order_id": TEST_ORDER_ID,
        "printer_type": TEST_PRINTER_TYPE,
        "print_invoice_type": 1,
    }

    # 2. JSON Serialization
    # Python's json.dumps with indent=0 adds newlines!
    api_data_json_indent0 = json.dumps(business_params, indent=0)

    # Compact JSON (no indent, no newlines) - this is what JS JSON.stringify produces
    api_data_json_compact = json.dumps(business_params, separators=(',', ':'))

    # Default json.dumps (with spaces after : and ,)
    api_data_json_default = json.dumps(business_params)

    print("=" * 60)
    print("PYTHON OUTPUT - FOR TYPESCRIPT COMPARISON")
    print("=" * 60)

    print("\n--- 1. Business Parameters ---")
    print(f"order_id: {TEST_ORDER_ID}")
    print(f"printer_type: {TEST_PRINTER_TYPE}")

    print("\n--- 2. JSON Serialization Variants ---")
    print(f"\n[A] json.dumps(indent=0) (what Python code uses):")
    print(repr(api_data_json_indent0))
    print(f"Length: {len(api_data_json_indent0)}")

    print(
        f"\n[B] json.dumps(separators=(',', ':')) (compact, like JS JSON.stringify):")
    print(repr(api_data_json_compact))
    print(f"Length: {len(api_data_json_compact)}")

    print(f"\n[C] json.dumps() default:")
    print(repr(api_data_json_default))
    print(f"Length: {len(api_data_json_default)}")

    # 3. Signature Generation for each variant
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
        print(f"  Signature Source: {repr(sig_source)}")
        print(f"  MD5 Signature: {sig}")

    # 4. Form Data
    print("\n--- 4. Form Data Fields ---")
    print(f"invoice: {INVOICE_TAX_ID}")
    print(f"data: (from JSON above)")
    print(f"time: {FIXED_TIMESTAMP}")
    print(f"sign: (from signature above)")

    print("\n" + "=" * 60)
    print("RECOMMENDATION:")
    print("=" * 60)
    print("""
The Python code uses `json.dumps(params, indent=0)` which produces:
  - Newlines between key-value pairs
  - This is DIFFERENT from JavaScript's JSON.stringify()

To align TypeScript with Python, either:
  A) Change Python to use compact JSON: json.dumps(params, separators=(',', ':'))
  B) Change TypeScript to mimic indent=0 behavior (add newlines)

Since the API is already working with Python, TypeScript should match Python's output.
""")


if __name__ == "__main__":
    generate_test_values()
