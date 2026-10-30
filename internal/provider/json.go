package provider

import (
	"bytes"
	"encoding/json"
	"reflect"
)

// SameJSON reports whether two JSON documents hold the same value, whatever their
// spacing and key order. Invalid JSON is never the same as anything.
func SameJSON(a, b []byte) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	return reflect.DeepEqual(x, y)
}

// CompactJSON is a document without spacing, for storing in state.
func CompactJSON(raw []byte) string {
	var buf bytes.Buffer
	if json.Compact(&buf, raw) != nil {
		return string(raw)
	}
	return buf.String()
}

// ValidJSONObject reports whether raw is a JSON object.
func ValidJSONObject(raw string) bool {
	var v map[string]any
	return json.Unmarshal([]byte(raw), &v) == nil
}
