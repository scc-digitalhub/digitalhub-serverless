/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package tvm

import (
	"encoding/binary"
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

// v2ToDLPack maps an Open Inference v2 datatype to a DLPack (code, bits) pair.
// Only the native dtypes this serve image supports are accepted; FP16/BOOL are
// deferred and rejected here (mirrors the rust worker).
func v2ToDLPack(datatype string) (code, bits uint8, err error) {
	switch datatype {
	case "FP32":
		return dlFloat, 32, nil
	case "FP64":
		return dlFloat, 64, nil
	case "INT8":
		return dlInt, 8, nil
	case "INT16":
		return dlInt, 16, nil
	case "INT32":
		return dlInt, 32, nil
	case "INT64":
		return dlInt, 64, nil
	case "UINT8":
		return dlUInt, 8, nil
	case "UINT16":
		return dlUInt, 16, nil
	case "UINT32":
		return dlUInt, 32, nil
	case "UINT64":
		return dlUInt, 64, nil
	case "FP16":
		return 0, 0, fmt.Errorf("datatype 'FP16' is not supported by this serve image (deferred)")
	default:
		return 0, 0, fmt.Errorf("unsupported datatype %q", datatype)
	}
}

// supportedTvmDtype reports whether a metadata.json TVM dtype string (e.g.
// "float32", "int64") is a native dtype this serve image can handle.
func supportedTvmDtype(dt string) bool {
	switch dt {
	case "float32", "float64",
		"int8", "int16", "int32", "int64",
		"uint8", "uint16", "uint32", "uint64":
		return true
	}
	return false
}

// dlpackToV2 maps a DLPack (code, bits) pair back to a v2 datatype string.
func dlpackToV2(code, bits int) (string, error) {
	switch {
	case code == dlFloat && bits == 32:
		return "FP32", nil
	case code == dlFloat && bits == 64:
		return "FP64", nil
	case code == dlInt && bits == 8:
		return "INT8", nil
	case code == dlInt && bits == 16:
		return "INT16", nil
	case code == dlInt && bits == 32:
		return "INT32", nil
	case code == dlInt && bits == 64:
		return "INT64", nil
	case code == dlUInt && bits == 8:
		return "UINT8", nil
	case code == dlUInt && bits == 16:
		return "UINT16", nil
	case code == dlUInt && bits == 32:
		return "UINT32", nil
	case code == dlUInt && bits == 64:
		return "UINT64", nil
	default:
		return "", fmt.Errorf("unsupported output dtype (code=%d bits=%d); FP16/others deferred", code, bits)
	}
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
// read the JSON integer directly so INT64 keeps full precision.
func encodeInputBytes(nums []json.Number, datatype string) ([]byte, error) {
	switch datatype {
	case "FP32":
		b := make([]byte, len(nums)*4)
		for i, n := range nums {
			f, err := n.Float64()
			if err != nil {
				return nil, err
			}
			binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(float32(f)))
		}
		return b, nil
	case "FP64":
		b := make([]byte, len(nums)*8)
		for i, n := range nums {
			f, err := n.Float64()
			if err != nil {
				return nil, err
			}
			binary.LittleEndian.PutUint64(b[i*8:], math.Float64bits(f))
		}
		return b, nil
	case "INT8":
		b := make([]byte, len(nums))
		for i, n := range nums {
			v, err := n.Int64()
			if err != nil {
				return nil, err
			}
			b[i] = byte(int8(v))
		}
		return b, nil
	case "INT16":
		b := make([]byte, len(nums)*2)
		for i, n := range nums {
			v, err := n.Int64()
			if err != nil {
				return nil, err
			}
			binary.LittleEndian.PutUint16(b[i*2:], uint16(int16(v)))
		}
		return b, nil
	case "INT32":
		b := make([]byte, len(nums)*4)
		for i, n := range nums {
			v, err := n.Int64()
			if err != nil {
				return nil, err
			}
			binary.LittleEndian.PutUint32(b[i*4:], uint32(int32(v)))
		}
		return b, nil
	case "INT64":
		b := make([]byte, len(nums)*8)
		for i, n := range nums {
			v, err := n.Int64()
			if err != nil {
				return nil, err
			}
			binary.LittleEndian.PutUint64(b[i*8:], uint64(v))
		}
		return b, nil
	case "UINT8":
		b := make([]byte, len(nums))
		for i, n := range nums {
			u, err := numUint64(n)
			if err != nil {
				return nil, err
			}
			b[i] = byte(u)
		}
		return b, nil
	case "UINT16":
		b := make([]byte, len(nums)*2)
		for i, n := range nums {
			u, err := numUint64(n)
			if err != nil {
				return nil, err
			}
			binary.LittleEndian.PutUint16(b[i*2:], uint16(u))
		}
		return b, nil
	case "UINT32":
		b := make([]byte, len(nums)*4)
		for i, n := range nums {
			u, err := numUint64(n)
			if err != nil {
				return nil, err
			}
			binary.LittleEndian.PutUint32(b[i*4:], uint32(u))
		}
		return b, nil
	case "UINT64":
		b := make([]byte, len(nums)*8)
		for i, n := range nums {
			u, err := numUint64(n)
			if err != nil {
				return nil, err
			}
			binary.LittleEndian.PutUint64(b[i*8:], u)
		}
		return b, nil
	default:
		return nil, fmt.Errorf("unsupported datatype %q (FP16/others deferred)", datatype)
	}
}

// decodeOutputData decodes raw little-endian bytes of the given v2 datatype into a
// typed slice that json.Marshal renders as an array of numbers. UINT8 uses []int
// (not []uint8/[]byte, which json would base64-encode).
func decodeOutputData(b []byte, datatype string) (any, error) {
	switch datatype {
	case "FP32":
		out := make([]float32, len(b)/4)
		for i := range out {
			out[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
		}
		return out, nil
	case "FP64":
		out := make([]float64, len(b)/8)
		for i := range out {
			out[i] = math.Float64frombits(binary.LittleEndian.Uint64(b[i*8:]))
		}
		return out, nil
	case "INT8":
		out := make([]int8, len(b))
		for i := range out {
			out[i] = int8(b[i])
		}
		return out, nil
	case "INT16":
		out := make([]int16, len(b)/2)
		for i := range out {
			out[i] = int16(binary.LittleEndian.Uint16(b[i*2:]))
		}
		return out, nil
	case "INT32":
		out := make([]int32, len(b)/4)
		for i := range out {
			out[i] = int32(binary.LittleEndian.Uint32(b[i*4:]))
		}
		return out, nil
	case "INT64":
		out := make([]int64, len(b)/8)
		for i := range out {
			out[i] = int64(binary.LittleEndian.Uint64(b[i*8:]))
		}
		return out, nil
	case "UINT8":
		out := make([]int, len(b))
		for i := range out {
			out[i] = int(b[i])
		}
		return out, nil
	case "UINT16":
		out := make([]uint16, len(b)/2)
		for i := range out {
			out[i] = binary.LittleEndian.Uint16(b[i*2:])
		}
		return out, nil
	case "UINT32":
		out := make([]uint32, len(b)/4)
		for i := range out {
			out[i] = binary.LittleEndian.Uint32(b[i*4:])
		}
		return out, nil
	case "UINT64":
		out := make([]uint64, len(b)/8)
		for i := range out {
			out[i] = binary.LittleEndian.Uint64(b[i*8:])
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported output datatype %q", datatype)
	}
}
