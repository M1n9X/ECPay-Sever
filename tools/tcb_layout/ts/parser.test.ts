import * as assert from "assert";
import { findLayout, loadDefaultLayouts, parse } from "./parser";

function setField(buf: Buffer, pos: number, len: number, value: string) {
  const start = pos - 1;
  const end = start + len;
  buf.fill(" ", start, end);
  buf.write(value, start, "utf8");
}

function testLayout311() {
  const layouts = loadDefaultLayouts();
  const layout = findLayout(layouts, "3.1.1");
  assert.ok(layout, "layout 3.1.1 not found");

  const payload = Buffer.alloc(600, " ");
  setField(payload, 1, 2, "01");
  setField(payload, 3, 2, "02");
  setField(payload, 5, 6, "ABC123");
  setField(payload, 34, 12, "000000010000");
  setField(payload, 83, 8, "TID12345");

  const parsed = parse(layout!, payload);
  assert.strictEqual(parsed["Trans_Type"].raw, "01");
  assert.strictEqual(parsed["Host_ID"].raw, "02");
  assert.strictEqual(parsed["Invoice_No"].trimmed, "ABC123");
  assert.strictEqual(parsed["Trans_Amount"].raw, "000000010000");
  assert.strictEqual(parsed["EDC_Terminal_ID"].trimmed, "TID12345");
}

function testLayout314() {
  const layouts = loadDefaultLayouts();
  const layout = findLayout(layouts, "3.1.14");
  assert.ok(layout, "layout 3.1.14 not found");

  const payload = Buffer.alloc(600, " ");
  setField(payload, 1, 2, "36");
  setField(payload, 34, 12, "000000000500");
  setField(payload, 58, 20, "ORDER123");
  setField(payload, 78, 1, "1");
  setField(payload, 79, 4, "0000");

  const parsed = parse(layout!, payload);
  assert.strictEqual(parsed["Trans_Type"].raw, "36");
  assert.strictEqual(parsed["Order_Num"].trimmed, "ORDER123");
  assert.strictEqual(parsed["Order_Status"].raw, "1");
  assert.strictEqual(parsed["ECR_Response_Code"].raw, "0000");
}

function run() {
  testLayout311();
  testLayout314();
  console.log("parser tests passed");
}

run();
