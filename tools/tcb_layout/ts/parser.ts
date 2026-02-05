import * as fs from "fs";
import * as path from "path";

export interface FieldDef {
  pos: number;
  len: number;
  field: string;
  desc: string;
}

export interface LayoutDef {
  id: string;
  name: string;
  trans_types?: string[];
  notes?: string;
  fields: FieldDef[];
}

export interface Spec {
  layouts: LayoutDef[];
}

export interface Value {
  raw: string;
  trimmed: string;
}

export type Parsed = Record<string, Value>;

export function loadLayouts(jsonPath: string): LayoutDef[] {
  const raw = fs.readFileSync(jsonPath, "utf8");
  const spec = JSON.parse(raw) as Spec;
  return spec.layouts;
}

export function findLayout(layouts: LayoutDef[], id: string): LayoutDef | undefined {
  return layouts.find((l) => l.id === id);
}

export function parse(layout: LayoutDef, data: Uint8Array): Parsed {
  if (data.length !== 600) {
    throw new Error("payload length must be 600");
  }
  const out: Parsed = {};
  for (const f of layout.fields) {
    const start = f.pos - 1;
    const end = start + f.len;
    if (start < 0 || end > data.length) {
      throw new Error("field slice out of range");
    }
    const raw = Buffer.from(data.slice(start, end)).toString("utf8");
    out[f.field] = { raw, trimmed: raw.replace(/\s+$/g, "") };
  }
  return out;
}

export function loadDefaultLayouts(): LayoutDef[] {
  return loadLayouts(path.resolve(__dirname, "../../../docs/tcb_layout_v36.json"));
}
