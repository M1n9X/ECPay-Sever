####
# 參考範例 API
# https://invoice.amego.tw/api_doc/#api-%E7%99%BC%E7%A5%A8-Invoice_Print
####

import requests
import json
import time
import hashlib
from typing import Dict, Any, Optional

# Configuration
API_URL = "https://invoice-api.amego.tw/json/invoice_print"
APP_KEY = "sHeq7t8G1wiQvhAuIM27"
INVOICE_TAX_ID = "12345678"  # 統編


def fetch_print_data(order_id: str, printer_type: int = 2) -> Optional[str]:
    """
    Fetch the base64 print data from Amego Invoice API for a given order.

    Args:
        order_id: The order ID to query.
        printer_type: 2 for generic ESC/POS (returns base64).

    Returns:
        Base64 encoded print string if successful, None otherwise.
    """

    # 1. Prepare Business Parameters
    # These parameters are serialized and signed
    business_params = {
        "type": "order",             # Query by order ID
        "order_id": order_id,
        "printer_type": printer_type,
        "print_invoice_type": 1,     # 1 = Original Invoice, 2 = Re-print
    }

    # 2. Serialize to JSON string
    # Using indent=0 to match reference implementation requirement (compact JSON)
    api_data_json = json.dumps(business_params, indent=0)

    # 3. Generate Request Signature
    current_timestamp = int(time.time())

    # Signature = MD5( api_data_json + timestamp + app_key )
    signature_source = f"{api_data_json}{current_timestamp}{APP_KEY}"
    signature = hashlib.md5(signature_source.encode("utf-8")).hexdigest()

    # 4. Construct Final Request Payload
    # The API expects form-urlencoded body with these specific keys
    request_payload = {
        "invoice": INVOICE_TAX_ID,
        "data": api_data_json,
        "time": current_timestamp,
        "sign": signature
    }

    headers = {
        'Content-Type': 'application/x-www-form-urlencoded'
    }

    print(f"Sending request to: {API_URL}")
    print(f"Payload Data: {business_params}")

    try:
        response = requests.post(
            API_URL, headers=headers, data=request_payload)
        response.raise_for_status()

        resp_json = response.json()

        # Check API business code
        code = resp_json.get('code')
        if code == 0:
            data = resp_json.get('data', {})
            base64_data = data.get('base64_data')

            if base64_data:
                print(
                    f"\n[SUCCESS] Received Base64 Data (Length: {len(base64_data)})")
                return base64_data
            else:
                print("\n[WARNING] 'base64_data' field missing in response.")
                print(f"Response data: {data}")
                return None
        else:
            print(f"\n[ERROR] API returned error code: {code}")
            print(f"Message: {resp_json.get('msg')}")
            # print(f"Full Response: {json.dumps(resp_json, indent=2, ensure_ascii=False)}")
            return None

    except requests.exceptions.RequestException as e:
        print(f"\n[CRITICAL] HTTP Request failed: {e}")
        return None
    except json.JSONDecodeError:
        print(f"\n[CRITICAL] Failed to decode JSON response.")
        print(f"Raw text: {response.text}")
        return None


if __name__ == "__main__":
    # Example usage
    TEST_ORDER_ID = "A20200817106955"

    result = fetch_print_data(TEST_ORDER_ID)

    if result:
        print(f"Base64 Data Snippet: {result[:50]}...")
