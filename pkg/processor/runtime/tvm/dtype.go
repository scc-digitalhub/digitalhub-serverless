/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package tvm

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
)

// DLPack type codes (dlpack.h DLDataTypeCode).
const (
	dlInt   = 0
	dlUInt  = 1
	dlFloat = 2
)

// dtypeInfo describes one supported dtype: its DLPack (code, bits) pair and the
// TVM dtype string used in metadata.json (e.g. "float32").
type dtypeInfo struct {
	code, bits uint8
	tvmName    string
}

// dtypeTable is the single source of truth for the native dtypes this serve
// image supports, keyed by the Open Inference v2 datatype name; every mapping
// and codec below derives from it. FP16/BOOL are deferred and rejected
// (mirrors the rust worker).
var dtypeTable = map[string]dtypeInfo{
	"FP32":   {dlFloat, 32, "float32"},
	"FP64":   {dlFloat, 64, "float64"},
	"INT8":   {dlInt, 8, "int8"},
	"INT16":  {dlInt, 16, "int16"},
	"INT32":  {dlInt, 32, "int32"},
	"INT64":  {dlInt, 64, "int64"},
	"UINT8":  {dlUInt, 8, "uint8"},
	"UINT16": {dlUInt, 16, "uint16"},
	"UINT32": {dlUInt, 32, "uint32"},
	"UINT64": {dlUInt, 64, "uint64"},
}

// lookups derived from dtypeTable: the metadata.json TVM dtype names, and the
// reverse (code, bits) -> v2 datatype map.
var (
	tvmDtypeNames = func() map[string]bool {
		m := make(map[string]bool, len(dtypeTable))
		for _, info := range dtypeTable {
			m[info.tvmName] = true
		}
		return m
	}()
	dlpackToV2Name = func() map[[2]int]string {
		m := make(map[[2]int]string, len(dtypeTable))
		for name, info := range dtypeTable {
			m[[2]int{int(info.code), int(info.bits)}] = name
		}
		return m
	}()
)

// v2ToDLPack maps an Open Inference v2 datatype to a DLPack (code, bits) pair.
// Only the native dtypes this serve image supports are accepted; FP16/BOOL are
// deferred and rejected here (mirrors the rust worker).
func v2ToDLPack(datatype string) (code, bits uint8, err error) {
	if datatype == "FP16" {
		return 0, 0, fmt.Errorf("datatype 'FP16' is not supported by this serve image (deferred)")
	}
	info, ok := dtypeTable[datatype]
	if !ok {
		return 0, 0, fmt.Errorf("unsupported datatype %q", datatype)
	}
	return info.code, info.bits, nil
}

// supportedTvmDtype reports whether a metadata.json TVM dtype string (e.g.
// "float32", "int64") is a native dtype this serve image can handle.
func supportedTvmDtype(dt string) bool {
	return tvmDtypeNames[dt]
}

// dlpackToV2 maps a DLPack (code, bits) pair back to a v2 datatype string.
func dlpackToV2(code, bits int) (string, error) {
	if name, ok := dlpackToV2Name[[2]int{code, bits}]; ok {
		return name, nil
	}
	return "", fmt.Errorf("unsupported output dtype (code=%d bits=%d); FP16/others deferred", code, bits)
}

// collectNumbers flattens a JSON-decoded numeric tensor payload (flat or nested
// arrays) into a flat slice of json.Number, preserving integer precision (the
// request is decoded with UseNumber). float64 leaves are accepted as a fallback.
func collectNumbers(v any) ([]json.Number, error) {
	var out []json.Number
	var rec func(any) error
	rec = func(x any) error {
		switch t := x.(type) {
		case []any:
			for _, e := range t {
				if err := rec(e); err != nil {
					return err
				}
			}
		case json.Number:
			out = append(out, t)
		case float64:
			out = append(out, json.Number(strconv.FormatFloat(t, 'g', -1, 64)))
		case nil:
			return fmt.Errorf("nil data element")
		default:
			return fmt.Errorf("unsupported data element type %T", x)
		}
		return nil
	}
	if err := rec(v); err != nil {
		return nil, err
	}
	return out, nil
}

func numUint64(n json.Number) (uint64, error) {
	if u, err := strconv.ParseUint(n.String(), 10, 64); err == nil {
		return u, nil
	}
	f, err := n.Float64()
	if err != nil {
		return 0, err
	}
	return uint64(f), nil
}

// encodeInputBytes encodes flattened numbers into raw little-endian bytes for the
// given v2 datatype, ready to memcpy into a DLTensor of that dtype. Integer types
// read the JSON integer directly so INT64 keeps full precision. Every element is
// funneled through its uint64 bit pattern and the low bits/8 bytes are written
// little-endian: two's-complement truncation makes that identical to the
// per-dtype casts (e.g. byte(int8(v)) == low byte of uint64(v)).
func encodeInputBytes(nums []json.Number, datatype string) ([]byte, error) {
	info, ok := dtypeTable[datatype]
	if !ok {
		// defensively dead: v2ToDLPack already validated the datatype upstream.
		return nil, fmt.Errorf("unsupported datatype %q (FP16/others deferred)", datatype)
	}
	width := int(info.bits) / 8
	b := make([]byte, len(nums)*width)
	for i, n := range nums {
		var u uint64
		switch info.code {
		case dlFloat:
			f, err := n.Float64()
			if err != nil {
				return nil, err
			}
			if info.bits == 32 {
				u = uint64(math.Float32bits(float32(f)))
			} else {
				u = math.Float64bits(f)
			}
		case dlInt:
			v, err := n.Int64()
			if err != nil {
				return nil, err
			}
			u = uint64(v)
		default: // dlUInt
			v, err := numUint64(n)
			if err != nil {
				return nil, err
			}
			u = v
		}
		for k := 0; k < width; k++ {
			b[i*width+k] = byte(u >> (8 * k))
		}
	}
	return b, nil
}

// decodeLE decodes raw little-endian bytes into a typed slice, reading `width`
// bytes per element into a uint64 bit pattern that conv turns into the value.
func decodeLE[T any](b []byte, width int, conv func(uint64) T) []T {
	out := make([]T, len(b)/width)
	for i := range out {
		var u uint64
		for k := 0; k < width; k++ {
			u |= uint64(b[i*width+k]) << (8 * k)
		}
		out[i] = conv(u)
	}
	return out
}

// decodeOutputData decodes raw little-endian bytes of the given v2 datatype into a
// typed slice that json.Marshal renders as an array of numbers. UINT8 uses []int
// (not []uint8/[]byte, which json would base64-encode).
func decodeOutputData(b []byte, datatype string) (any, error) {
	switch datatype {
	case "FP32":
		return decodeLE(b, 4, func(u uint64) float32 { return math.Float32frombits(uint32(u)) }), nil
	case "FP64":
		return decodeLE(b, 8, math.Float64frombits), nil
	case "INT8":
		return decodeLE(b, 1, func(u uint64) int8 { return int8(u) }), nil
	case "INT16":
		return decodeLE(b, 2, func(u uint64) int16 { return int16(u) }), nil
	case "INT32":
		return decodeLE(b, 4, func(u uint64) int32 { return int32(u) }), nil
	case "INT64":
		return decodeLE(b, 8, func(u uint64) int64 { return int64(u) }), nil
	case "UINT8":
		return decodeLE(b, 1, func(u uint64) int { return int(u) }), nil
	case "UINT16":
		return decodeLE(b, 2, func(u uint64) uint16 { return uint16(u) }), nil
	case "UINT32":
		return decodeLE(b, 4, func(u uint64) uint32 { return uint32(u) }), nil
	case "UINT64":
		return decodeLE(b, 8, func(u uint64) uint64 { return u }), nil
	default:
		// defensively dead: dlpackToV2 only ever produces the names above.
		return nil, fmt.Errorf("unsupported output datatype %q", datatype)
	}
}
