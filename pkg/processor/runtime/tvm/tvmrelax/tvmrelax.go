/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

// Package tvmrelax is a cgo binding that loads a compiled TVM Relax model.so and
// runs inference, driving it through raw PackedFunc calls over TVM's tvm-ffi C ABI
// (TVM ships no high-level Relax-VM binding). Mirrors the rust `tvm-relax` crate.
// The Go side only exchanges raw little-endian bytes + a DLPack dtype (code, bits)
// + shapes; the C helpers below do the ABI work. LLVM CPU and optional CUDA; FP16 deferred.
//
// NOT thread-safe: the VM and every tvm-ffi handle must be used from ONE OS thread.
//
// Build needs the tvm-ffi/dlpack headers and libtvm_ffi.so + libtvm_runtime.so,
// supplied via CGO_CFLAGS / CGO_LDFLAGS.
package tvmrelax

/*
#cgo LDFLAGS: -Wl,--no-as-needed -ltvm_runtime -Wl,--as-needed -ltvm_ffi
#include <tvm/ffi/c_api.h>
#include <dlpack/dlpack.h>
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

#define TVM_MAX_OUT 64

// tvm-ffi ABI: everything is a refcounted TVMFFIObjectHandle, invoked via
// TVMFFIFunctionCall(fn, args, n, &ret) where each arg/result is a TVMFFIAny tagged
// union. A call returns an OWNED handle in ret.v_obj that must be DecRef'd. Tensors
// cross as DLPack DLManagedTensors (TensorFromDLPack in, TensorToDLPack out).
typedef struct {
  TVMFFIObjectHandle lib;
  TVMFFIObjectHandle vm;
  TVMFFIObjectHandle entry;
  TVMFFIObjectHandle fGetFn;   // ffi.ModuleGetFunction
  TVMFFIObjectHandle fArrGet;  // ffi.ArrayGetItem  (multi-output: read array element i)
  TVMFFIObjectHandle fArrSize; // ffi.ArraySize     (multi-output: array length)
  TVMFFIObjectHandle fShape;
  TVMFFIObjectHandle fAlloc;
  TVMFFIObjectHandle fCopyFrom;
  TVMFFIObjectHandle fCopyTo;
  TVMFFIObjectHandle fSync;
  DLDevice device;
} tvm_model_t;

typedef struct {
  TVMFFIObjectHandle h[TVM_MAX_OUT]; // input ffi.Tensor handles
  int n;
} tvm_inputs_t;

typedef struct {
  DLManagedTensor* outs[TVM_MAX_OUT]; // output managed tensors (owned)
  int n;
} tvm_result_t;

// pull the last raised tvm-ffi error into buf, prefixed by ctx.
static void tvm_take_error(const char* ctx, char* buf, int cap) {
  TVMFFIObjectHandle err = NULL;
  TVMFFIErrorMoveFromRaised(&err);
  if (err) {
    // C equivalent of the C++-only TVMFFIErrorGetCellPtr: cell sits right after the header.
    TVMFFIErrorCell* cell = (TVMFFIErrorCell*)((char*)err + sizeof(TVMFFIObject));
    if (cell->message.data && cell->message.size > 0) {
      snprintf(buf, cap, "%s: %.*s", ctx, (int)cell->message.size, cell->message.data);
      TVMFFIObjectDecRef(err);
      return;
    }
    TVMFFIObjectDecRef(err);
  }
  snprintf(buf, cap, "%s: tvm-ffi call failed", ctx);
}

// accept only serveable dtypes (lanes==1; float 32/64, int/uint 8/16/32/64); FP16 deferred.
static int tvm_check_out_supported(tvm_result_t* res, int i, char* err, int cap) {
  DLDataType dt = res->outs[i]->dl_tensor.dtype;
  int ok = dt.lanes == 1 && (
    (dt.code == kDLFloat && (dt.bits == 32 || dt.bits == 64)) ||
    (dt.code == kDLInt   && (dt.bits == 8 || dt.bits == 16 || dt.bits == 32 || dt.bits == 64)) ||
    (dt.code == kDLUInt  && (dt.bits == 8 || dt.bits == 16 || dt.bits == 32 || dt.bits == 64)));
  if (!ok) {
    snprintf(err, cap, "output[%d] dtype (code=%d bits=%d lanes=%d) unsupported (FP16/others deferred)",
             i, (int)dt.code, (int)dt.bits, (int)dt.lanes);
    return -1;
  }
  return 0;
}

static TVMFFIObjectHandle tvm_get_global(const char* name) {
  TVMFFIByteArray ba; ba.data = name; ba.size = strlen(name);
  TVMFFIObjectHandle h = NULL;
  if (TVMFFIFunctionGetGlobal(&ba, &h) != 0) return NULL;
  return h;
}

// ModuleGetFunction(mod, name, query_imports=true) -> function handle
static int tvm_modgetfn(tvm_model_t* m, TVMFFIObjectHandle mod, const char* name,
                        TVMFFIObjectHandle* out, char* err, int cap) {
  *out = NULL;
  TVMFFIAny a[3], r;
  memset(a, 0, sizeof(a));
  a[0].type_index = kTVMFFIModule; a[0].v_obj = (TVMFFIObject*)mod;
  a[1].type_index = kTVMFFIRawStr; a[1].v_c_str = name;
  a[2].type_index = kTVMFFIBool;   a[2].v_int64 = 1;
  r.type_index = kTVMFFINone;
  if (TVMFFIFunctionCall(m->fGetFn, a, 3, &r) != 0) { tvm_take_error(name, err, cap); return -1; }
  if (r.type_index != kTVMFFIFunction || !r.v_obj) {
    snprintf(err, cap, "required function %s is missing or invalid", name);
    if (r.type_index >= kTVMFFIStaticObjectBegin && r.v_obj) TVMFFIObjectDecRef(r.v_obj);
    return -1;
  }
  *out = (TVMFFIObjectHandle)r.v_obj;
  return 0;
}

// Release all handles on the same OS thread that owns the VM.
static void tvm_model_free(tvm_model_t* m) {
  TVMFFIObjectHandle handles[] = {m->entry, m->vm, m->lib, m->fGetFn,
    m->fArrGet, m->fArrSize, m->fShape, m->fAlloc, m->fCopyFrom, m->fCopyTo, m->fSync};
  for (unsigned i = 0; i < sizeof(handles) / sizeof(handles[0]); i++) {
    if (handles[i]) TVMFFIObjectDecRef(handles[i]);
  }
  memset(m, 0, sizeof(*m));
}

static int tvm_device_sync(tvm_model_t* m, char* err, int cap) {
  TVMFFIAny a[2], r; memset(a, 0, sizeof(a)); memset(&r, 0, sizeof(r));
  a[0].type_index = kTVMFFIDevice; a[0].v_device = m->device;
  a[1].type_index = kTVMFFIInt; a[1].v_int64 = 0;
  if (TVMFFIFunctionCall(m->fSync, a, 2, &r) != 0) {
    tvm_take_error("Device_StreamSync", err, cap); return -1;
  }
  return 0;
}

static int tvm_check_cuda(char* err, int cap) {
  TVMFFIObjectHandle backend = tvm_get_global("device_api.cuda");
  if (!backend) {
    snprintf(err, cap, "CUDA backend is missing: libtvm_runtime_cuda.so must be built, packaged and loaded");
    return -1;
  }
  TVMFFIObjectDecRef(backend);
  TVMFFIObjectHandle attr = tvm_get_global("runtime.GetDeviceAttr");
  if (!attr) { snprintf(err, cap, "missing runtime.GetDeviceAttr"); return -1; }
  TVMFFIAny a[3], r; memset(a, 0, sizeof(a)); memset(&r, 0, sizeof(r));
  a[0].type_index = kTVMFFIInt; a[0].v_int64 = kDLCUDA;
  a[1].type_index = kTVMFFIInt; a[1].v_int64 = 0;
  a[2].type_index = kTVMFFIInt; a[2].v_int64 = 0; // kExist
  int rc = TVMFFIFunctionCall(attr, a, 3, &r);
  TVMFFIObjectDecRef(attr);
  if (rc != 0) { tvm_take_error("CUDA device 0 availability", err, cap); return -1; }
  if (r.v_int64 == 0) {
    snprintf(err, cap, "CUDA device 0 is unavailable: expose an NVIDIA GPU and a compatible driver to the container");
    return -1;
  }
  return 0;
}

// Load model.so, initialize the selected device plus CPU host, and resolve entry.
static int tvm_model_load(const char* so_path, const char* entry, int device_type,
                          tvm_model_t* m, char* err, int cap) {
  memset(m, 0, sizeof(*m));
  m->device.device_type = (DLDeviceType)device_type;
  m->device.device_id = 0;
  if (device_type == kDLCUDA && tvm_check_cuda(err, cap) != 0) return -1;
  m->fGetFn  = tvm_get_global("ffi.ModuleGetFunction");
  m->fArrGet = tvm_get_global("ffi.ArrayGetItem");
  m->fArrSize= tvm_get_global("ffi.ArraySize");
  m->fShape  = tvm_get_global("ffi.Shape");
  m->fAlloc  = tvm_get_global("runtime.TVMTensorAllocWithScope");
  m->fCopyFrom = tvm_get_global("runtime.TVMTensorCopyFromBytes");
  m->fCopyTo = tvm_get_global("runtime.TVMTensorCopyToBytes");
  m->fSync = tvm_get_global("runtime.Device_StreamSync");
  if (!m->fGetFn || !m->fArrGet || !m->fArrSize || !m->fShape || !m->fAlloc ||
      !m->fCopyFrom || !m->fCopyTo || !m->fSync) {
    snprintf(err, cap, "missing required TVM runtime globals"); return -1;
  }
  TVMFFIObjectHandle fLoad = tvm_get_global("ffi.ModuleLoadFromFile");
  if (!fLoad) { snprintf(err, cap, "missing ffi.ModuleLoadFromFile"); return -1; }

  // lib = ffi.ModuleLoadFromFile(so_path)
  TVMFFIAny a, r;
  memset(&a, 0, sizeof(a)); a.type_index = kTVMFFIRawStr; a.v_c_str = so_path;
  r.type_index = kTVMFFINone;
  int load_rc = TVMFFIFunctionCall(fLoad, &a, 1, &r);
  TVMFFIObjectDecRef(fLoad);
  if (load_rc != 0) { tvm_take_error("ModuleLoadFromFile", err, cap); return -1; }
  m->lib = (TVMFFIObjectHandle)r.v_obj;

  // loader = lib["vm_load_executable"]; vm = loader()
  TVMFFIObjectHandle loader;
  if (tvm_modgetfn(m, m->lib, "vm_load_executable", &loader, err, cap) != 0) return -1;
  r.type_index = kTVMFFINone;
  if (TVMFFIFunctionCall(loader, NULL, 0, &r) != 0) { tvm_take_error("vm_load_executable", err, cap); TVMFFIObjectDecRef(loader); return -1; }
  m->vm = (TVMFFIObjectHandle)r.v_obj;
  TVMFFIObjectDecRef(loader);

  // vm["vm_initialization"](device_type,0,kPooled, kDLCPU,0,kPooled)
  TVMFFIObjectHandle vmInit;
  if (tvm_modgetfn(m, m->vm, "vm_initialization", &vmInit, err, cap) != 0) return -1;
  TVMFFIAny ia[6]; memset(ia, 0, sizeof(ia));
  // each triple = (device_type, device_id, allocator_type); 2 = kPooled
  long devs[6] = {device_type, 0, 2, kDLCPU, 0, 2};
  for (int i = 0; i < 6; i++) { ia[i].type_index = kTVMFFIInt; ia[i].v_int64 = devs[i]; }
  r.type_index = kTVMFFINone;
  if (TVMFFIFunctionCall(vmInit, ia, 6, &r) != 0) { tvm_take_error("vm_initialization", err, cap); TVMFFIObjectDecRef(vmInit); return -1; }
  TVMFFIObjectDecRef(vmInit);

  // entry = vm[entry]
  if (tvm_modgetfn(m, m->vm, entry, &m->entry, err, cap) != 0) return -1;
  return 0;
}

// Allocate on the selected device using TVM, then synchronously copy host bytes.
// TVM owns both storage and tensor metadata; no custom CUDA allocator is needed.
static int tvm_input_add(tvm_model_t* m, tvm_inputs_t* in, const void* data, long long nbytes,
                         unsigned char code, unsigned char bits, int ndim,
                         const long long* shape, char* err, int cap) {
  if (in->n >= TVM_MAX_OUT) { snprintf(err, cap, "too many inputs"); return -1; }
  TVMFFIAny* dims = (TVMFFIAny*)calloc(ndim > 0 ? (size_t)ndim : 1, sizeof(TVMFFIAny));
  if (!dims) { snprintf(err, cap, "failed to allocate input shape"); return -1; }
  for (int k = 0; k < ndim; k++) {
    dims[k].type_index = kTVMFFIInt; dims[k].v_int64 = shape[k];
  }
  TVMFFIAny r; memset(&r, 0, sizeof(r));
  int rc = TVMFFIFunctionCall(m->fShape, dims, ndim, &r);
  free(dims);
  if (rc != 0) { tvm_take_error("ffi.Shape", err, cap); return -1; }
  TVMFFIObjectHandle shape_obj = (TVMFFIObjectHandle)r.v_obj;
  TVMFFIAny a[4]; memset(a, 0, sizeof(a));
  a[0].type_index = kTVMFFIShape; a[0].v_obj = (TVMFFIObject*)shape_obj;
  a[1].type_index = kTVMFFIDataType;
  a[1].v_dtype.code = code; a[1].v_dtype.bits = bits; a[1].v_dtype.lanes = 1;
  a[2].type_index = kTVMFFIDevice; a[2].v_device = m->device;
  a[3].type_index = kTVMFFINone; // default memory scope
  memset(&r, 0, sizeof(r));
  rc = TVMFFIFunctionCall(m->fAlloc, a, 4, &r);
  TVMFFIObjectDecRef(shape_obj);
  if (rc != 0) { tvm_take_error("TVMTensorAllocWithScope", err, cap); return -1; }
  TVMFFIObjectHandle tensor = (TVMFFIObjectHandle)r.v_obj;
  TVMFFIAny ca[3]; memset(ca, 0, sizeof(ca));
  ca[0].type_index = kTVMFFITensor; ca[0].v_obj = (TVMFFIObject*)tensor;
  ca[1].type_index = kTVMFFIOpaquePtr; ca[1].v_ptr = (void*)data;
  ca[2].type_index = kTVMFFIInt; ca[2].v_int64 = nbytes;
  memset(&r, 0, sizeof(r));
  rc = TVMFFIFunctionCall(m->fCopyFrom, ca, 3, &r);
  if (rc != 0) {
    tvm_take_error("TVMTensorCopyFromBytes", err, cap);
    TVMFFIObjectDecRef(tensor); return -1;
  }
  in->h[in->n++] = tensor;
  return 0;
}

static void tvm_inputs_free(tvm_inputs_t* in) {
  for (int i = 0; i < in->n; i++) if (in->h[i]) TVMFFIObjectDecRef(in->h[i]);
  in->n = 0;
}

// run entry(inputs...) -> Tensor or Array<Tensor>; store output managed tensors.
static int tvm_model_run(tvm_model_t* m, tvm_inputs_t* in, tvm_result_t* out,
                         char* err, int cap) {
  out->n = 0;
  TVMFFIAny args[TVM_MAX_OUT], r;
  for (int i = 0; i < in->n; i++) {
    memset(&args[i], 0, sizeof(args[i]));
    args[i].type_index = kTVMFFITensor;
    args[i].v_obj = (TVMFFIObject*)in->h[i];
  }
  r.type_index = kTVMFFINone;
  int rc = TVMFFIFunctionCall(m->entry, args, in->n, &r);
  // Keep inputs alive until queued device work has completed, including errors.
  if (rc != 0) {
    tvm_take_error("entry", err, cap);
    char sync_err[256];
    tvm_device_sync(m, sync_err, sizeof(sync_err));
    return -1;
  }
  if (tvm_device_sync(m, err, cap) != 0) {
    if (r.type_index >= kTVMFFIStaticObjectBegin) TVMFFIObjectDecRef((TVMFFIObjectHandle)r.v_obj);
    return -1;
  }

  if (r.type_index == kTVMFFITensor) {
    TVMFFIObjectHandle h = (TVMFFIObjectHandle)r.v_obj;
    if (TVMFFITensorToDLPack(h, &out->outs[0]) != 0) { tvm_take_error("ToDLPack", err, cap); TVMFFIObjectDecRef(h); return -1; }
    TVMFFIObjectDecRef(h);
    out->n = 1;
    if (tvm_check_out_supported(out, 0, err, cap) != 0) return -1; // caller frees via tvm_result_free
    return 0;
  }
  if (r.type_index == kTVMFFIArray) {
    TVMFFIObjectHandle arr = (TVMFFIObjectHandle)r.v_obj;
    TVMFFIAny aa, ra; memset(&aa, 0, sizeof(aa));
    aa.type_index = kTVMFFIArray; aa.v_obj = (TVMFFIObject*)arr; ra.type_index = kTVMFFINone;
    if (TVMFFIFunctionCall(m->fArrSize, &aa, 1, &ra) != 0) { tvm_take_error("ArraySize", err, cap); TVMFFIObjectDecRef(arr); return -1; }
    if (ra.v_int64 < 0 || ra.v_int64 > TVM_MAX_OUT) {
      snprintf(err, cap, "unsupported output count %lld (maximum %d)", (long long)ra.v_int64, TVM_MAX_OUT);
      TVMFFIObjectDecRef(arr);
      return -1;
    }
    int n = (int)ra.v_int64;
    for (int i = 0; i < n; i++) {
      TVMFFIAny ga[2], gr; memset(ga, 0, sizeof(ga));
      ga[0].type_index = kTVMFFIArray; ga[0].v_obj = (TVMFFIObject*)arr;
      ga[1].type_index = kTVMFFIInt;   ga[1].v_int64 = i;
      gr.type_index = kTVMFFINone;
      if (TVMFFIFunctionCall(m->fArrGet, ga, 2, &gr) != 0) { tvm_take_error("ArrayGetItem", err, cap); TVMFFIObjectDecRef(arr); return -1; }
      if (gr.type_index != kTVMFFITensor || !gr.v_obj) {
        snprintf(err, cap, "output[%d] must be a Tensor (type_index=%d)", i, gr.type_index);
        if (gr.type_index >= kTVMFFIStaticObjectBegin && gr.v_obj) TVMFFIObjectDecRef(gr.v_obj);
        TVMFFIObjectDecRef(arr);
        return -1;
      }
      TVMFFIObjectHandle th = (TVMFFIObjectHandle)gr.v_obj;
      if (TVMFFITensorToDLPack(th, &out->outs[out->n]) != 0) { tvm_take_error("ToDLPack[i]", err, cap); TVMFFIObjectDecRef(th); TVMFFIObjectDecRef(arr); return -1; }
      TVMFFIObjectDecRef(th);
      out->n++;
      if (tvm_check_out_supported(out, out->n - 1, err, cap) != 0) { TVMFFIObjectDecRef(arr); return -1; }
    }
    TVMFFIObjectDecRef(arr);
    return 0;
  }
  snprintf(err, cap, "unexpected output type_index %d", r.type_index);
  if (r.type_index >= kTVMFFIStaticObjectBegin && r.v_obj) TVMFFIObjectDecRef(r.v_obj);
  return -1;
}

static int     tvm_out_ndim(tvm_result_t* res, int i) { return res->outs[i]->dl_tensor.ndim; }
static long long tvm_out_dim(tvm_result_t* res, int i, int k) { return (long long)res->outs[i]->dl_tensor.shape[k]; }
static int       tvm_out_dtype_code(tvm_result_t* res, int i) { return (int)res->outs[i]->dl_tensor.dtype.code; }
static int       tvm_out_dtype_bits(tvm_result_t* res, int i) { return (int)res->outs[i]->dl_tensor.dtype.bits; }
static long long tvm_out_nbytes(tvm_result_t* res, int i) {
  DLTensor* t = &res->outs[i]->dl_tensor; long long n = 1;
  for (int k = 0; k < t->ndim; k++) n *= t->shape[k];
  return n * (long long)((t->dtype.bits + 7) / 8);
}
// copy the output's raw LE bytes into dst (sized by the caller via tvm_out_nbytes).
static int tvm_out_copy(tvm_model_t* m, tvm_result_t* res, int i, void* dst,
                        long long nbytes, char* err, int cap) {
  TVMFFIAny a[3], r; memset(a, 0, sizeof(a)); memset(&r, 0, sizeof(r));
  a[0].type_index = kTVMFFIDLTensorPtr; a[0].v_ptr = &res->outs[i]->dl_tensor;
  a[1].type_index = kTVMFFIOpaquePtr; a[1].v_ptr = dst;
  a[2].type_index = kTVMFFIInt; a[2].v_int64 = nbytes;
  if (TVMFFIFunctionCall(m->fCopyTo, a, 3, &r) != 0) {
    tvm_take_error("TVMTensorCopyToBytes", err, cap); return -1;
  }
  return 0;
}
static void tvm_result_free(tvm_result_t* res) {
  for (int i = 0; i < res->n; i++) {
    DLManagedTensor* m = res->outs[i];
    if (m && m->deleter) m->deleter(m);
    res->outs[i] = NULL;
  }
  res->n = 0;
}
*/
import "C"

import (
	"fmt"
	"runtime"
	"unsafe"
)

// Tensor contains host bytes for a row-major tensor. Data holds raw LE element bytes for the DLPack
// dtype (Code, Bits), so any dtype crosses the cgo boundary the same way. Positional
// (no names cross here); the v2 <-> (code, bits) mapping lives in package tvm.
type Tensor struct {
	Shape []int64
	Code  uint8
	Bits  uint8
	Data  []byte
}

// Model holds the loaded Relax VM + cached entry function. Not thread-safe.
type Model struct {
	c C.tvm_model_t
}

// Load loads an LLVM CPU model. All model operations must use the same OS thread.
func Load(soPath, entry string) (*Model, error) {
	return LoadWithBackend(soPath, entry, "llvm")
}

// LoadWithBackend initializes device 0 of the backend declared by model metadata.
// CUDA requires the cuda build tag and a visible NVIDIA device; it never falls back.
func LoadWithBackend(soPath, entry, backend string) (*Model, error) {
	device := C.int(C.kDLCPU)
	switch backend {
	case "llvm":
	case "cuda":
		if !cudaEnabled {
			return nil, fmt.Errorf("CUDA model requires a CUDA-enabled serve image (build tag cuda)")
		}
		device = C.int(C.kDLCUDA)
	default:
		return nil, fmt.Errorf("unsupported TVM backend %q", backend)
	}
	cso := C.CString(soPath)
	defer C.free(unsafe.Pointer(cso))
	cen := C.CString(entry)
	defer C.free(unsafe.Pointer(cen))

	m := &Model{}
	errbuf := make([]C.char, 256)
	if C.tvm_model_load(cso, cen, device, &m.c, &errbuf[0], 256) != 0 {
		C.tvm_model_free(&m.c)
		return nil, fmt.Errorf("tvm load: %s", C.GoString(&errbuf[0]))
	}
	return m, nil
}

// Close releases the VM and its handles on the owning OS thread.
func (m *Model) Close() {
	C.tvm_model_free(&m.c)
}

// Infer runs the entry function with the given inputs (positional) and returns
// the output tensors, each carrying its raw little-endian bytes plus DLPack dtype
// (Code, Bits). Must run on the Model's OS thread.
func (m *Model) Infer(inputs []Tensor) ([]Tensor, error) {
	var in C.tvm_inputs_t
	in.n = 0
	defer C.tvm_inputs_free(&in)
	errbuf := make([]C.char, 256)

	for i := range inputs {
		t := &inputs[i]
		var dptr unsafe.Pointer
		if len(t.Data) > 0 {
			dptr = unsafe.Pointer(&t.Data[0])
		}
		var sptr *C.longlong
		if len(t.Shape) > 0 {
			sptr = (*C.longlong)(unsafe.Pointer(&t.Shape[0]))
		}
		rc := C.tvm_input_add(&m.c, &in, dptr, C.longlong(len(t.Data)), C.uchar(t.Code), C.uchar(t.Bits),
			C.int(len(t.Shape)), sptr, &errbuf[0], 256)
		runtime.KeepAlive(t.Data)
		runtime.KeepAlive(t.Shape)
		if rc != 0 {
			return nil, fmt.Errorf("tvm input[%d]: %s", i, C.GoString(&errbuf[0]))
		}
	}

	var res C.tvm_result_t
	// free outputs even on the error path: the multi-output case can collect some
	// tensors (tracked in res.n) before failing, so defer before the error check.
	defer C.tvm_result_free(&res)
	if C.tvm_model_run(&m.c, &in, &res, &errbuf[0], 256) != 0 {
		return nil, fmt.Errorf("tvm infer: %s", C.GoString(&errbuf[0]))
	}

	nout := int(res.n)
	outs := make([]Tensor, nout)
	for i := 0; i < nout; i++ {
		nd := int(C.tvm_out_ndim(&res, C.int(i)))
		shape := make([]int64, nd)
		for k := 0; k < nd; k++ {
			shape[k] = int64(C.tvm_out_dim(&res, C.int(i), C.int(k)))
		}
		nbytes := int(C.tvm_out_nbytes(&res, C.int(i)))
		data := make([]byte, nbytes)
		if nbytes > 0 {
			if C.tvm_out_copy(&m.c, &res, C.int(i), unsafe.Pointer(&data[0]), C.longlong(nbytes), &errbuf[0], 256) != 0 {
				return nil, fmt.Errorf("tvm output[%d]: %s", i, C.GoString(&errbuf[0]))
			}
		}
		outs[i] = Tensor{
			Shape: shape,
			Code:  uint8(C.tvm_out_dtype_code(&res, C.int(i))),
			Bits:  uint8(C.tvm_out_dtype_bits(&res, C.int(i))),
			Data:  data,
		}
	}
	return outs, nil
}
