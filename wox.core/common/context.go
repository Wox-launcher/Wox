package common

import "encoding/json"

type ContextData map[string]string

func (c ContextData) Marshal() string {
	if len(c) == 0 {
		return ""
	}
	data, err := json.Marshal(c)
	if err != nil {
		return ""
	}
	return string(data)
}

func UnmarshalContextData(raw string) ContextData {
	if raw == "" {
		return ContextData{}
	}

	var data ContextData
	if err := json.Unmarshal([]byte(raw), &data); err == nil {
		return data
	}

	return ContextData{}
}
