package protocol

import (
	"fmt"
	"strings"
	"testing"
)

func TestLayoutSchema(t *testing.T) {
	spec, err := loadLayouts()
	if err != nil {
		t.Fatalf("load layouts: %v", err)
	}
	if len(spec.Layouts) == 0 {
		t.Fatalf("no layouts found")
	}

	for _, layout := range spec.Layouts {
		if len(layout.Fields) == 0 {
			t.Fatalf("layout %s has no fields", layout.ID)
		}
		allowOverlap := layoutAllowsOverlap(layout)
		occupied := make(map[int]string)

		for _, f := range layout.Fields {
			if f.Pos < 1 {
				t.Fatalf("layout %s field %s has invalid pos %d", layout.ID, f.Field, f.Pos)
			}
			if f.Len <= 0 {
				t.Fatalf("layout %s field %s has invalid len %d", layout.ID, f.Field, f.Len)
			}
			end := f.Pos + f.Len - 1
			if end > PayloadLen {
				t.Fatalf("layout %s field %s exceeds payload: end=%d", layout.ID, f.Field, end)
			}

			if allowOverlap {
				continue
			}

			for i := f.Pos; i <= end; i++ {
				if prev, ok := occupied[i]; ok {
					t.Fatalf("layout %s overlap at pos %d: %s vs %s", layout.ID, i, prev, f.Field)
				}
				occupied[i] = f.Field
			}
		}
	}
}

func TestBuildParseRoundTrip(t *testing.T) {
	spec, err := loadLayouts()
	if err != nil {
		t.Fatalf("load layouts: %v", err)
	}

	for _, layout := range spec.Layouts {
		if len(layout.TransTypes) == 0 {
			t.Fatalf("layout %s has no trans_types", layout.ID)
		}
		transType := layout.TransTypes[0]
		overlapFields := detectOverlapFields(layout)

		fields := make(map[string]string)
		expected := make(map[string]string)
		fieldLen := make(map[string]int)

		for _, f := range layout.Fields {
			if f.Field == "" || f.Len <= 0 {
				continue
			}
			fieldLen[f.Field] = f.Len

			val := sampleValue(f.Len, padTypeForField(f.Field))
			fields[f.Field] = val
			expected[f.Field] = applyPad(f.Len, val, padTypeForField(f.Field))
		}

		if l, ok := fieldLen["Trans_Type"]; ok {
			fields["Trans_Type"] = transType
			expected["Trans_Type"] = applyPad(l, transType, padTypeForField("Trans_Type"))
		}

		req := TCBRequest{TransType: transType, Fields: fields}
		packet, err := BuildPacket(req)
		if err != nil {
			t.Fatalf("layout %s build packet: %v", layout.ID, err)
		}
		if !ValidatePacket(packet) {
			t.Fatalf("layout %s packet failed validation", layout.ID)
		}

		resp, err := ParseResponse(packet)
		if err != nil {
			t.Fatalf("layout %s parse response: %v", layout.ID, err)
		}

		for name, exp := range expected {
			if overlapFields[name] {
				continue
			}
			got, ok := resp[name]
			if !ok {
				t.Fatalf("layout %s missing field %s in response", layout.ID, name)
			}
			expTrim := strings.TrimRight(exp, " ")
			if got != expTrim {
				t.Fatalf("layout %s field %s mismatch: expected=%q got=%q", layout.ID, name, expTrim, got)
			}
		}
	}
}

func layoutAllowsOverlap(layout LayoutDef) bool {
	note := strings.ToLower(layout.Notes)
	if strings.Contains(note, "overlap") {
		return true
	}
	if strings.Contains(layout.Notes, "重叠") || strings.Contains(layout.Notes, "重疊") {
		return true
	}
	return false
}

func detectOverlapFields(layout LayoutDef) map[string]bool {
	overlaps := make(map[string]bool)
	occupied := make(map[int]string)

	for _, f := range layout.Fields {
		start := f.Pos
		end := f.Pos + f.Len - 1
		for i := start; i <= end; i++ {
			if prev, ok := occupied[i]; ok {
				overlaps[prev] = true
				overlaps[f.Field] = true
				continue
			}
			occupied[i] = f.Field
		}
	}
	return overlaps
}

func sampleValue(length int, padType string) string {
	if length <= 0 {
		return ""
	}
	count := length
	if count > 4 {
		count = 4
	}
	if padType == "LEFT_ZERO" {
		return strings.Repeat("1", count)
	}
	return strings.Repeat("A", count)
}

func applyPad(length int, val string, padType string) string {
	if len(val) > length {
		val = val[:length]
	}
	if padType == "LEFT_ZERO" {
		return fmt.Sprintf("%0*s", length, val)
	}
	return fmt.Sprintf("%-*s", length, val)
}
