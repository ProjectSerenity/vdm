// Copyright 2026 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

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
