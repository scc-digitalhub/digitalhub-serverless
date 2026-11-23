/*
SPDX-FileCopyrightText: © 2026 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package tvmrelax

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The fixtures are real Relax executables; keep architecture-specific binaries
// outside the source tree. Both VM modes must reject invalid FFI values safely.
func TestInvalidModelFunctionsAndOutputs(t *testing.T) {
	dir := os.Getenv("TVM_TEST_FIXTURES")
	if dir == "" {
		t.Skip("set TVM_TEST_FIXTURES to the Relax regression fixture directory")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	for _, mode := range []string{"bytecode", "compiled"} {
		path := filepath.Join(dir, "mixed-primitive-"+mode, "model.so")
		for attempt := 0; attempt < 20; attempt++ {
			m, err := Load(path, "missing_entry")
			if m != nil || err == nil || !strings.Contains(err.Error(), "missing_entry") {
				t.Fatalf("%s missing entry: model=%v error=%v", mode, m, err)
			}
		}
		for _, kind := range []string{"mixed-primitive", "nested-tuple", "primitive"} {
			m, err := Load(filepath.Join(dir, kind+"-"+mode, "model.so"), "main")
			if err != nil {
				t.Fatal(err)
			}
			data := make([]byte, 16)
			for i := 0; i < 4; i++ {
				binary.LittleEndian.PutUint32(data[4*i:], math.Float32bits(float32(i+1)))
			}
			for attempt := 0; attempt < 20; attempt++ {
				_, err := m.Infer([]Tensor{{Shape: []int64{4}, Code: 2, Bits: 32, Data: data}})
				if err == nil {
					m.Close()
					t.Fatalf("%s/%s accepted unsupported output", mode, kind)
				}
			}
			m.Close()
		}
	}
}
