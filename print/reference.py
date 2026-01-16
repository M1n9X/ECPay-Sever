####
# 參考範例 API
# https://invoice.amego.tw/api_doc/#api-%E7%99%BC%E7%A5%A8-Invoice
####
import hashlib
import requests
import time
import urllib
import json

APP_KEY = "sHeq7t8G1wiQvhAuIM27"

# B2C - 開立發票 + 列印
sUrl = "https://invoice-api.amego.tw/json/f0401"

# Unix Timesatmp 10位數，不含毫秒
nCurrent_Now_Time = int(time.time())  # 1628136135

invoice_data = {
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

# Convert Python to JSON
sApi_Data = json.dumps(invoice_data, indent=0)
# print(sApi_Data)

# 此範例 md5 結果為 f53a336934b2af0589b845638d1495cc，請自檢測是否相符
sHash_Text = sApi_Data + str(nCurrent_Now_Time) + APP_KEY
# print(sHash_Text)
m = hashlib.md5()
m.update(sHash_Text.encode("utf-8"))
sSign = m.hexdigest()
# print(sSign)

aPost_Data = {
    "invoice": '12345678',  # 統編
    "data": sApi_Data,
    "time": nCurrent_Now_Time,
    "sign": sSign,
}
# print(aPost_Data)

# 將資料內容進行 url encode
payload = urllib.parse.urlencode(aPost_Data, doseq=True)
# print(payload)

headers = {
    'Content-Type': 'application/x-www-form-urlencoded'
}

response = requests.request("POST", sUrl, headers=headers, data=payload)

print(json.loads(response.text))
