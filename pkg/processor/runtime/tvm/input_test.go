package tvm

import (
	"encoding/json"
	"math"
	"testing"

	nuclio "github.com/nuclio/nuclio-sdk-go"
	"github.com/scc-digitalhub/digitalhub-serverless/pkg/processor/trigger/openinference"
	"github.com/stretchr/testify/require"
)

type bodyEvent struct {
	nuclio.Event
	body []byte
}

func (e bodyEvent) GetBody() []byte { return e.body }

func TestInputSignature(t *testing.T) {
	r := &tvmRuntime{metadata: modelMetadata{Inputs: []tensorSpec{
		{Name: "x", Shape: []int64{1, 2}, Dtype: "float32"},
	}}}
	valid := openinference.RESTInferInputTensor{
		Name: "x", Shape: []int64{1, 2}, Datatype: "FP32",
		Data: []any{json.Number("1"), json.Number("2")},
	}
	for _, tc := range []struct {
		name string
		edit func(*openinference.RESTInferInputTensor)
	}{
		{"wrong shape", func(i *openinference.RESTInferInputTensor) { i.Shape = []int64{2, 1} }},
		{"wrong rank", func(i *openinference.RESTInferInputTensor) { i.Shape = []int64{2} }},
		{"wrong dtype", func(i *openinference.RESTInferInputTensor) { i.Datatype = "INT32" }},
		{"negative shape", func(i *openinference.RESTInferInputTensor) { i.Shape = []int64{-1, 2} }},
		{"missing values", func(i *openinference.RESTInferInputTensor) { i.Data = []any{} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := valid
			tc.edit(&input)
			body, err := json.Marshal(openinference.RESTInferenceRequest{Inputs: []openinference.RESTInferInputTensor{input}})
			require.NoError(t, err)
			// A validation failure must return before accessing the uninitialized worker.
			_, err = r.buildInputs([]openinference.RESTInferInputTensor{input})
			require.Error(t, err)
			_, err = r.ProcessEvent(bodyEvent{body: body}, nil)
			var invalid *openinference.InvalidInputError
			require.ErrorAs(t, err, &invalid)
		})
	}
	t.Run("positional fallback", func(t *testing.T) {
		input := valid
		input.Name = "unknown"
		_, err := r.buildInputs([]openinference.RESTInferInputTensor{input})
		require.NoError(t, err)
	})
	t.Run("dynamic dimension", func(t *testing.T) {
		r := &tvmRuntime{metadata: modelMetadata{Inputs: []tensorSpec{{Name: "x", Shape: []int64{-1, 2}, Dtype: "float32"}}}}
		_, err := r.buildInputs([]openinference.RESTInferInputTensor{valid})
		require.NoError(t, err)
	})
}

func TestMultiInputOrderingAndPositionalFallback(t *testing.T) {
	r := &tvmRuntime{metadata: modelMetadata{Inputs: []tensorSpec{
		{Name: "x", Shape: []int64{1}, Dtype: "float32"},
		{Name: "y", Shape: []int64{2}, Dtype: "int32"},
	}}}
	x := openinference.RESTInferInputTensor{Name: "x", Datatype: "FP32", Shape: []int64{1}, Data: []any{json.Number("1")}}
	y := openinference.RESTInferInputTensor{Name: "y", Datatype: "INT32", Shape: []int64{2}, Data: []any{json.Number("2"), json.Number("3")}}
	ordered, err := r.buildInputs([]openinference.RESTInferInputTensor{y, x})
	require.NoError(t, err)
	require.Equal(t, []int64{1}, ordered[0].Shape)
	for _, name := range []string{"", "unknown", "y"} {
		x.Name = name
		_, err = r.buildInputs([]openinference.RESTInferInputTensor{x, y})
		require.NoError(t, err)
	}
}

func TestInputNumericBounds(t *testing.T) {
	for _, tc := range []struct{ dtype, value string }{
		{"INT8", "128"}, {"INT8", "-129"}, {"UINT8", "256"}, {"UINT8", "-1"},
		{"INT16", "32768"}, {"UINT16", "65536"}, {"INT32", "2147483648"},
		{"UINT32", "4294967296"}, {"INT64", "9223372036854775808"},
		{"UINT64", "18446744073709551616"}, {"INT8", "1.5"}, {"UINT8", "1.5"},
		{"FP32", "1e100"}, {"FP64", "1e1000"},
		{"FP32", "NaN"}, {"FP32", "+Inf"}, {"FP32", "-Inf"},
		{"FP64", "NaN"}, {"FP64", "+Inf"}, {"FP64", "-Inf"},
		{"INT8", "1e-1000000000"}, {"INT8", "1e1000000000"},
		{"INT64", "9223372036854775807.1"}, {"UINT64", "18446744073709551615.1"},
	} {
		t.Run(tc.dtype+"/"+tc.value, func(t *testing.T) {
			_, err := encodeInputBytes([]json.Number{json.Number(tc.value)}, tc.dtype)
			require.Error(t, err)
		})
	}
	for _, dtype := range []string{"INT8", "UINT8", "INT64", "UINT64"} {
		_, err := encodeInputBytes([]json.Number{"1.0", "1e1", "0.0", "-0e1000000000"}, dtype)
		require.NoError(t, err)
	}
}

func TestInputArity(t *testing.T) {
	input := openinference.RESTInferInputTensor{Shape: []int64{1}, Data: []any{json.Number("1")}}
	for _, count := range []int{0, 1, 2} {
		r := &tvmRuntime{metadata: modelMetadata{Inputs: make([]tensorSpec, count)}}
		for _, supplied := range []int{0, 1, 2} {
			inputs := make([]openinference.RESTInferInputTensor, supplied)
			for i := range inputs {
				inputs[i] = input
			}
			_, err := r.buildInputs(inputs)
			if count == supplied {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, "input tensor(s)")
			}
		}
	}
}

func TestInputShapeElementCount(t *testing.T) {
	r := &tvmRuntime{}
	for _, tc := range []struct {
		shape []int64
		data  []any
		valid bool
	}{
		{[]int64{math.MaxInt64, 2}, []any{}, false},
		{[]int64{-1}, []any{}, false},
		{[]int64{2}, []any{json.Number("1")}, false},
		{[]int64{}, []any{json.Number("1")}, true},
		{[]int64{0}, []any{}, true},
	} {
		_, err := r.buildInputs([]openinference.RESTInferInputTensor{{Shape: tc.shape, Data: tc.data}})
		if tc.valid {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}
	}
}

func TestIntegerDecimalPrecision(t *testing.T) {
	for _, tc := range []struct {
		dtype string
		value json.Number
		want  any
	}{
		{"INT64", "9223372036854775807.0", []int64{math.MaxInt64}},
		{"INT64", "-9.223372036854775808e18", []int64{math.MinInt64}},
		{"UINT64", "1.8446744073709551615e19", []uint64{math.MaxUint64}},
	} {
		raw, err := encodeInputBytes([]json.Number{tc.value}, tc.dtype)
		require.NoError(t, err)
		got, err := decodeOutputData(raw, tc.dtype)
		require.NoError(t, err)
		require.Equal(t, tc.want, got)
	}
}
