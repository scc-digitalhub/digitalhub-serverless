#!/usr/bin/env bash
# Generate the Nuclio processor config from env, then run the processor. The
# native `tvm` runtime loads the model from TVM_MODEL_DIR; the openinference
# trigger exposes OpenInference v2 on the configured ports.
set -euo pipefail

CFG=/tmp/processor.yaml
MODEL_DIR="${TVM_MODEL_DIR:-/shared/model}"

# Populate the v2 metadata endpoints (/v2/models/<name>, gRPC ModelMetadata) from
# the model's metadata.json: the trigger serves input_tensors/output_tensors from
# its config only, so without this clients would see empty inputs/outputs.
# YAML is a JSON superset, so compact JSON arrays can be embedded directly.
IN_T="[]"; OUT_T="[]"
if command -v jq >/dev/null 2>&1 && [ -f "$MODEL_DIR/metadata.json" ]; then
    DT_MAP='{"float32":"FP32","float64":"FP64","float16":"FP16","int64":"INT64","int32":"INT32","int16":"INT16","int8":"INT8","uint8":"UINT8","uint16":"UINT16","uint32":"UINT32","uint64":"UINT64","bool":"BOOL"}'
    IN_T=$(jq -c --argjson m "$DT_MAP" '[(.inputs // [])[] | {name, datatype: ($m[.dtype] // "FP32"), shape}]' "$MODEL_DIR/metadata.json" 2>/dev/null || echo "[]")
    OUT_T=$(jq -c --argjson m "$DT_MAP" '[(.outputs // [])[] | {name, datatype: ($m[.dtype] // "FP32"), shape}]' "$MODEL_DIR/metadata.json" 2>/dev/null || echo "[]")
fi

cat > "$CFG" <<EOF
apiVersion: "nuclio.io/v1"
kind: NuclioFunction
metadata:
  name: "${TVM_MODEL_NAME:-model}"
  namespace: nuclio
spec:
  runtime: tvm
  handler: tvm
  triggers:
    openinference:
      kind: openinference
      numWorkers: ${TVM_SERVE_WORKERS:-1}
      attributes:
        model_name: "${TVM_MODEL_NAME:-model}"
        enable_rest: true
        enable_grpc: true
        rest_port: ${TVM_SERVE_PORT:-8080}
        grpc_port: ${TVM_SERVE_GRPC_PORT:-9000}
        input_tensors: ${IN_T}
        output_tensors: ${OUT_T}
EOF

echo "[tvm-go-serve] model=${TVM_MODEL_NAME:-model} dir=${MODEL_DIR} rest=${TVM_SERVE_PORT:-8080} grpc=${TVM_SERVE_GRPC_PORT:-9000} workers=${TVM_SERVE_WORKERS:-1}"
exec /opt/nuclio/processor --config "$CFG"
