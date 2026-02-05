package protocol

import (
	_ "embed"
	"encoding/json"
	"sync"
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

//go:embed tcb_layout_v36.json
var layoutJSON []byte

var (
	layoutOnce sync.Once
	layoutSpec Spec
	layoutErr  error
	layoutByTT map[string]LayoutDef
)

func loadLayouts() (Spec, error) {
	layoutOnce.Do(func() {
		if err := json.Unmarshal(layoutJSON, &layoutSpec); err != nil {
			layoutErr = err
			return
		}
		layoutByTT = make(map[string]LayoutDef)
		for _, l := range layoutSpec.Layouts {
			for _, tt := range l.TransTypes {
				layoutByTT[tt] = l
			}
		}
	})
	return layoutSpec, layoutErr
}

func GetLayoutByTransType(transType string) (LayoutDef, bool, error) {
	_, err := loadLayouts()
	if err != nil {
		return LayoutDef{}, false, err
	}
	l, ok := layoutByTT[transType]
	return l, ok, nil
}
