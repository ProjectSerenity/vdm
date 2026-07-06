package vdm

import (
	"encoding/json"
)

func (p ParsedBool) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.Value)
}

func (p ParsedString) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.Value)
}
