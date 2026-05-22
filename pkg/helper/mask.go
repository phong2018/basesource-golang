package helper

import (
	"encoding/json"
	"strings"
)

var sensitiveKeys = map[string]bool{
	"password": true, "token": true, "access_token": true,
	"secret": true, "api_key": true, "authorization": true,
}

func MaskJSON(body string) string {
	if body == "" {
		return ""
	}
	var data any
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return body
	}
	result, err := json.Marshal(maskValue(data))
	if err != nil {
		return body
	}
	return string(result)
}

func maskValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, vv := range val {
			if sensitiveKeys[strings.ToLower(k)] {
				out[k] = "***"
			} else {
				out[k] = maskValue(vv)
			}
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, vv := range val {
			out[i] = maskValue(vv)
		}
		return out
	default:
		return v
	}
}
