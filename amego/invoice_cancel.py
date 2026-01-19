####
# 參考範例 API - 作廢發票
# https://invoice.amego.tw/api_doc/#api-%E7%99%BC%E7%A5%A8-Invoice_Cancel
####

import requests
import json
import time
import hashlib
from typing import Dict, Any, Optional

# Configuration
API_URL = "https://invoice-api.amego.tw/json/f0501"
APP_KEY = "sHeq7t8G1wiQvhAuIM27"
INVOICE_TAX_ID = "12345678"  # 統編


def cancel_invoice(invoice_number: str) -> Optional[Dict[str, Any]]:
    """
    Cancel an invoice using the Amego Invoice API.

    Args:
        invoice_number: The invoice number to cancel (e.g., "AB12345678")

    Returns:
        API response dict if successful, None otherwise.
    """

    # 1. Prepare Business Parameters
    # API expects a list of objects, each with 'CancelInvoiceNumber'
    business_params = [
        {
            "CancelInvoiceNumber": invoice_number
        }
    ]

    # 2. Serialize to JSON string
    # Using indent=0 to match reference implementation requirement
    api_data_json = json.dumps(business_params, indent=0)

    # 3. Generate Request Signature
    current_timestamp = int(time.time())

    # Signature = MD5( api_data_json + timestamp + app_key )
    signature_source = f"{api_data_json}{current_timestamp}{APP_KEY}"
    signature = hashlib.md5(signature_source.encode("utf-8")).hexdigest()

    # 4. Construct Final Request Payload
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
    print(f"Invoice Number: {invoice_number}")

    try:
        response = requests.post(
            API_URL, headers=headers, data=request_payload)
        response.raise_for_status()

        resp_json = response.json()

        # Check API business code
        code = resp_json.get('code')
        if code == 0:
            print(
                f"\n[SUCCESS] Invoice {invoice_number} cancelled successfully")
            return resp_json
        else:
            print(f"\n[ERROR] API returned error code: {code}")
            print(f"Message: {resp_json.get('msg')}")
            return resp_json

    except requests.exceptions.RequestException as e:
        print(f"\n[CRITICAL] HTTP Request failed: {e}")
        return None
    except json.JSONDecodeError:
        print(f"\n[CRITICAL] Failed to decode JSON response.")
        print(f"Raw text: {response.text}")
        return None


if __name__ == "__main__":
    # Example usage
    TEST_INVOICE_NUMBER = "AB12345678"
    result = cancel_invoice(TEST_INVOICE_NUMBER)

    if result:
        print(f"\nFull Response:")
        print(json.dumps(result, indent=2, ensure_ascii=False))
