package tcb_layout

import (
 "encoding/json"
 "errors"
 "os"
 "strings"
)

type FieldDef struct {
 Pos   int    `json:"pos"`
 Len   int    `json:"len"`
 Field string `json:"field"`
 Desc  string `json:"desc"`
}

type LayoutDef struct {
 ID         string     `json:"id"`
 Name       string     `json:"name"`
 TransTypes []string   `json:"trans_types,omitempty"`
 Notes      string     `json:"notes,omitempty"`
 Fields     []FieldDef `json:"fields"`
}

type Spec struct {
 Layouts []LayoutDef `json:"layouts"`
}

type Value struct {
 Raw     string
 Trimmed string
}

type Parsed map[string]Value

func LoadLayouts(path string) ([]LayoutDef, error) {
 b, err := os.ReadFile(path)
 if err != nil {
  return nil, err
 }
 var spec Spec
 if err := json.Unmarshal(b, &spec); err != nil {
  return nil, err
 }
 return spec.Layouts, nil
}

func FindLayout(layouts []LayoutDef, id string) (LayoutDef, bool) {
 for _, l := range layouts {
  if l.ID == id {
   return l, true
  }
 }
 return LayoutDef{}, false
}

func Parse(layout LayoutDef, data []byte) (Parsed, error) {
 if len(data) != 600 {
  return nil, errors.New("payload length must be 600")
 }
 out := make(Parsed, len(layout.Fields))
 for _, f := range layout.Fields {
  start := f.Pos - 1
  end := start + f.Len
  if start < 0 || end > len(data) {
   return nil, errors.New("field slice out of range")
  }
  raw := string(data[start:end])
  out[f.Field] = Value{
   Raw:     raw,
   Trimmed: strings.TrimRight(raw, " "),
  }
 }
 return out, nil
}
