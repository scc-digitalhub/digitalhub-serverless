/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

// Package tvmrelax is a native cgo binding that loads a compiled TVM Relax
// model.so and runs inference. It reproduces, in Go plus a little C glue, exactly
// what the rust `tvm-relax` crate does: TVM ships no high-level Relax-VM binding,
// so the model is driven through raw PackedFunc calls over TVM's tvm-ffi C ABI.
//
// The load + run dance is just five calls:
//
//	lib   = ffi.ModuleLoadFromFile("model.so")  // load the .so as a TVM Module
//	vm    = lib["vm_load_executable"]()         // -> the Relax VirtualMachine
//	        vm["vm_initialization"](CPU, CPU)    // init compute + host device (both CPU)
//	entry = vm[<entry>]                         // resolve the entry fn (usually "main")
//	out   = entry(inputs...)                    // run -> a Tensor or an Array<Tensor>
//
// All the fiddly ABI work (packing TVMFFIAny arguments, wrapping buffers as
// DLTensors, reference counting) is kept in the C helpers below, so the Go side
// only ever exchanges flat []float32 + shapes. CPU + FP32 only.
//
// NOT thread-safe: the VM and every tvm-ffi handle must be used from ONE OS thread.
// The caller pins a dedicated goroutine for Load+Infer (see the runtime's modelLoop).
//
// Build: needs the tvm-ffi/dlpack headers and libtvm_ffi.so + libtvm_runtime.so;
// supply the paths via CGO_CFLAGS / CGO_LDFLAGS, e.g.
//
//	CGO_CFLAGS="-I<tvm>/3rdparty/tvm-ffi/include -I<tvm>/3rdparty/tvm-ffi/3rdparty/dlpack/include"
//	CGO_LDFLAGS="-L<tvm>/build/lib -Wl,-rpath,/opt/tvm/lib"
package tvmrelax

/*
#cgo LDFLAGS: -Wl,--no-as-needed -ltvm_runtime -Wl,--as-needed -ltvm_ffi
#include <tvm/ffi/c_api.h>
#include <dlpack/dlpack.h>
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

#define TVM_MAX_OUT 64

// --- tvm-ffi ABI in one paragraph ---
// Everything (modules, functions, tensors, arrays) is a reference-counted
// TVMFFIObjectHandle. You invoke anything with TVMFFIFunctionCall(fn, args, n, &ret),
// where each arg and the result are a TVMFFIAny: a 16-byte tagged union
// {type_index, value}. A call returns an OWNED handle in ret.v_obj, so we must
// TVMFFIObjectDecRef() it once we are done. Named globals (e.g. "ffi.ModuleLoadFromFile")
// are resolved once via TVMFFIFunctionGetGlobal. Tensors cross the boundary as DLPack
// DLManagedTensors (TVMFFITensorFromDLPack in, TVMFFITensorToDLPack out).

// A loaded model: the DSO module, the Relax VM built from it, the cached entry
// function, plus the ffi globals we reuse per call. All must outlive inference.
typedef struct {
  TVMFFIObjectHandle lib;
  TVMFFIObjectHandle vm;
  TVMFFIObjectHandle entry;
  TVMFFIObjectHandle fGetFn;   // ffi.ModuleGetFunction
  TVMFFIObjectHandle fArrGet;  // ffi.ArrayGetItem  (multi-output: read array element i)
  TVMFFIObjectHandle fArrSize; // ffi.ArraySize     (multi-output: array length)
} tvm_model_t;

typedef struct {
  TVMFFIObjectHandle h[TVM_MAX_OUT]; // input ffi.Tensor handles
  int n;
} tvm_inputs_t;

typedef struct {
  DLManagedTensor* outs[TVM_MAX_OUT]; // output managed tensors (owned)
  int n;
} tvm_result_t;

// frees a DLManagedTensor whose data/shape were malloc'd by us (deleter route)
static void tvm_dl_free(struct DLManagedTensor* self) {
  if (self == NULL) return;
  if (self->dl_tensor.data) free(self->dl_tensor.data);
  if (self->dl_tensor.shape) free(self->dl_tensor.shape);
  free(self);
}

// pull the last raised tvm-ffi error into buf, prefixed by ctx. The error object
// carries the real message in its TVMFFIErrorCell; fall back to a generic label
// only when no error was raised or it has no message.
static void tvm_take_error(const char* ctx, char* buf, int cap) {
  TVMFFIObjectHandle err = NULL;
  TVMFFIErrorMoveFromRaised(&err);
  if (err) {
    // C equivalent of the C++-only inline TVMFFIErrorGetCellPtr: the error cell
    // sits immediately after the TVMFFIObject header.
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

// FP32-only contract: tvm_out_copy memcpy's raw bytes into a []float32, so any
// other output dtype would silently produce garbage (or read out of bounds for
// wider types). Reject with a clear error instead.
static int tvm_check_out_f32(tvm_result_t* res, int i, char* err, int cap) {
  DLDataType dt = res->outs[i]->dl_tensor.dtype;
  if (dt.code != kDLFloat || dt.bits != 32 || dt.lanes != 1) {
    snprintf(err, cap, "output[%d] dtype (code=%d bits=%d lanes=%d) unsupported: FP32 only",
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
  TVMFFIAny a[3], r;
  memset(a, 0, sizeof(a));
  a[0].type_index = kTVMFFIModule; a[0].v_obj = (TVMFFIObject*)mod;
  a[1].type_index = kTVMFFIRawStr; a[1].v_c_str = name;
  a[2].type_index = kTVMFFIBool;   a[2].v_int64 = 1;
  r.type_index = kTVMFFINone;
  if (TVMFFIFunctionCall(m->fGetFn, a, 3, &r) != 0) { tvm_take_error(name, err, cap); return -1; }
  *out = (TVMFFIObjectHandle)r.v_obj;
  return 0;
}

// load model.so + init Relax VM on CPU; resolve entry. Mirrors RelaxModel::load.
static int tvm_model_load(const char* so_path, const char* entry,
                          tvm_model_t* m, char* err, int cap) {
  memset(m, 0, sizeof(*m));
  TVMFFIObjectHandle fLoad = tvm_get_global("ffi.ModuleLoadFromFile");
  m->fGetFn  = tvm_get_global("ffi.ModuleGetFunction");
  m->fArrGet = tvm_get_global("ffi.ArrayGetItem");
  m->fArrSize= tvm_get_global("ffi.ArraySize");
  if (!fLoad || !m->fGetFn) { snprintf(err, cap, "missing tvm-ffi globals"); return -1; }

  // lib = ffi.ModuleLoadFromFile(so_path)
  TVMFFIAny a, r;
  memset(&a, 0, sizeof(a)); a.type_index = kTVMFFIRawStr; a.v_c_str = so_path;
  r.type_index = kTVMFFINone;
  if (TVMFFIFunctionCall(fLoad, &a, 1, &r) != 0) { tvm_take_error("ModuleLoadFromFile", err, cap); return -1; }
  m->lib = (TVMFFIObjectHandle)r.v_obj;

  // loader = lib["vm_load_executable"]; vm = loader()
  TVMFFIObjectHandle loader;
  if (tvm_modgetfn(m, m->lib, "vm_load_executable", &loader, err, cap) != 0) return -1;
  r.type_index = kTVMFFINone;
  if (TVMFFIFunctionCall(loader, NULL, 0, &r) != 0) { tvm_take_error("vm_load_executable", err, cap); TVMFFIObjectDecRef(loader); return -1; }
  m->vm = (TVMFFIObjectHandle)r.v_obj;
  TVMFFIObjectDecRef(loader);

  // vm["vm_initialization"](kDLCPU,0,kPooled, kDLCPU,0,kPooled)
  TVMFFIObjectHandle vmInit;
  if (tvm_modgetfn(m, m->vm, "vm_initialization", &vmInit, err, cap) != 0) return -1;
  TVMFFIAny ia[6]; memset(ia, 0, sizeof(ia));
  // each triple = (device_type, device_id, allocator_type); 2 = kPooled
  long devs[6] = {kDLCPU, 0, 2, kDLCPU, 0, 2};
  for (int i = 0; i < 6; i++) { ia[i].type_index = kTVMFFIInt; ia[i].v_int64 = devs[i]; }
  r.type_index = kTVMFFINone;
  if (TVMFFIFunctionCall(vmInit, ia, 6, &r) != 0) { tvm_take_error("vm_initialization", err, cap); TVMFFIObjectDecRef(vmInit); return -1; }
  TVMFFIObjectDecRef(vmInit);

  // entry = vm[entry]
  if (tvm_modgetfn(m, m->vm, entry, &m->entry, err, cap) != 0) return -1;
  return 0;
}

// add one FP32 input by COPYING data+shape into C memory wrapped as an ffi.Tensor
// (so Go memory need not outlive the call). type_index for the call arg = Tensor.
static int tvm_input_add(tvm_inputs_t* in, const float* data, int ndim,
                         const long long* shape, char* err, int cap) {
  if (in->n >= TVM_MAX_OUT) { snprintf(err, cap, "too many inputs"); return -1; }
  long long numel = 1;
  int64_t* shp = (int64_t*)malloc(sizeof(int64_t) * (ndim > 0 ? ndim : 1));
  for (int k = 0; k < ndim; k++) { shp[k] = (int64_t)shape[k]; numel *= shape[k]; }
  float* buf = (float*)malloc(sizeof(float) * (numel > 0 ? numel : 1));
  if (numel > 0) memcpy(buf, data, sizeof(float) * numel);

  DLManagedTensor* mt = (DLManagedTensor*)malloc(sizeof(DLManagedTensor));
  memset(mt, 0, sizeof(*mt));
  mt->dl_tensor.data = buf;
  mt->dl_tensor.device.device_type = kDLCPU;
  mt->dl_tensor.device.device_id = 0;
  mt->dl_tensor.ndim = ndim;
  mt->dl_tensor.dtype.code = kDLFloat;
  mt->dl_tensor.dtype.bits = 32;
  mt->dl_tensor.dtype.lanes = 1;
  mt->dl_tensor.shape = shp;
  mt->dl_tensor.strides = NULL;
  mt->dl_tensor.byte_offset = 0;
  mt->manager_ctx = NULL;
  mt->deleter = tvm_dl_free;

  TVMFFIObjectHandle h;
  if (TVMFFITensorFromDLPack(mt, 0, 0, &h) != 0) { tvm_take_error("TensorFromDLPack", err, cap); tvm_dl_free(mt); return -1; }
  in->h[in->n++] = h;
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
  tvm_inputs_free(in); // TVM held its own refs for the duration of the call
  if (rc != 0) { tvm_take_error("entry", err, cap); return -1; }

  if (r.type_index == kTVMFFITensor) {
    TVMFFIObjectHandle h = (TVMFFIObjectHandle)r.v_obj;
    if (TVMFFITensorToDLPack(h, &out->outs[0]) != 0) { tvm_take_error("ToDLPack", err, cap); TVMFFIObjectDecRef(h); return -1; }
    TVMFFIObjectDecRef(h);
    out->n = 1;
    if (tvm_check_out_f32(out, 0, err, cap) != 0) return -1; // caller frees via tvm_result_free
    return 0;
  }
  if (r.type_index == kTVMFFIArray) {
    TVMFFIObjectHandle arr = (TVMFFIObjectHandle)r.v_obj;
    TVMFFIAny aa, ra; memset(&aa, 0, sizeof(aa));
    aa.type_index = kTVMFFIArray; aa.v_obj = (TVMFFIObject*)arr; ra.type_index = kTVMFFINone;
    if (TVMFFIFunctionCall(m->fArrSize, &aa, 1, &ra) != 0) { tvm_take_error("ArraySize", err, cap); TVMFFIObjectDecRef(arr); return -1; }
    int n = (int)ra.v_int64;
    if (n > TVM_MAX_OUT) n = TVM_MAX_OUT;
    for (int i = 0; i < n; i++) {
      TVMFFIAny ga[2], gr; memset(ga, 0, sizeof(ga));
      ga[0].type_index = kTVMFFIArray; ga[0].v_obj = (TVMFFIObject*)arr;
      ga[1].type_index = kTVMFFIInt;   ga[1].v_int64 = i;
      gr.type_index = kTVMFFINone;
      if (TVMFFIFunctionCall(m->fArrGet, ga, 2, &gr) != 0) { tvm_take_error("ArrayGetItem", err, cap); TVMFFIObjectDecRef(arr); return -1; }
      TVMFFIObjectHandle th = (TVMFFIObjectHandle)gr.v_obj;
      if (TVMFFITensorToDLPack(th, &out->outs[out->n]) != 0) { tvm_take_error("ToDLPack[i]", err, cap); TVMFFIObjectDecRef(th); TVMFFIObjectDecRef(arr); return -1; }
      TVMFFIObjectDecRef(th);
      out->n++;
      if (tvm_check_out_f32(out, out->n - 1, err, cap) != 0) { TVMFFIObjectDecRef(arr); return -1; }
    }
    TVMFFIObjectDecRef(arr);
    return 0;
  }
  snprintf(err, cap, "unexpected output type_index %d", r.type_index);
  return -1;
}

static int     tvm_out_ndim(tvm_result_t* res, int i) { return res->outs[i]->dl_tensor.ndim; }
static long long tvm_out_dim(tvm_result_t* res, int i, int k) { return (long long)res->outs[i]->dl_tensor.shape[k]; }
static long long tvm_out_numel(tvm_result_t* res, int i) {
  DLTensor* t = &res->outs[i]->dl_tensor; long long n = 1;
  for (int k = 0; k < t->ndim; k++) n *= t->shape[k];
  return n;
}
static void tvm_out_copy(tvm_result_t* res, int i, float* dst) {
  DLTensor* t = &res->outs[i]->dl_tensor; long long n = 1;
  for (int k = 0; k < t->ndim; k++) n *= t->shape[k];
  if (n > 0) memcpy(dst, t->data, (size_t)n * sizeof(float));
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

// Tensor is a CPU/FP32 named tensor (row-major contiguous).
type Tensor struct {
	Name  string
	Shape []int64
	Data  []float32
}

// Model holds the loaded Relax VM + cached entry function. Not thread-safe.
type Model struct {
	c C.tvm_model_t
}

// Load loads model.so and initializes the Relax VM on CPU, resolving entry.
// Must be called on the same OS thread that will later call Infer.
func Load(soPath, entry string) (*Model, error) {
	cso := C.CString(soPath)
	defer C.free(unsafe.Pointer(cso))
	cen := C.CString(entry)
	defer C.free(unsafe.Pointer(cen))

	m := &Model{}
	errbuf := make([]C.char, 256)
	if C.tvm_model_load(cso, cen, &m.c, &errbuf[0], 256) != 0 {
		return nil, fmt.Errorf("tvm load: %s", C.GoString(&errbuf[0]))
	}
	return m, nil
}

// Infer runs the entry function with the given FP32 inputs (positional) and
// returns the output tensors. Must run on the Model's OS thread.
func (m *Model) Infer(inputs []Tensor) ([]Tensor, error) {
	var in C.tvm_inputs_t
	in.n = 0
	errbuf := make([]C.char, 256)

	for i := range inputs {
		t := &inputs[i]
		var dptr *C.float
		if len(t.Data) > 0 {
			dptr = (*C.float)(unsafe.Pointer(&t.Data[0]))
		}
		var sptr *C.longlong
		if len(t.Shape) > 0 {
			sptr = (*C.longlong)(unsafe.Pointer(&t.Shape[0]))
		}
		rc := C.tvm_input_add(&in, dptr, C.int(len(t.Shape)), sptr, &errbuf[0], 256)
		runtime.KeepAlive(t.Data)
		runtime.KeepAlive(t.Shape)
		if rc != 0 {
			C.tvm_inputs_free(&in)
			return nil, fmt.Errorf("tvm input[%d]: %s", i, C.GoString(&errbuf[0]))
		}
	}

	var res C.tvm_result_t
	// Free the output managed tensors even on the error path: in the multi-output
	// (Array) case tvm_model_run can collect some tensors before failing on a later
	// one, and it tracks the collected count in res.n (tvm_result_free is a no-op when
	// res.n == 0). Registering the defer BEFORE the error check plugs that leak.
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
		numel := int(C.tvm_out_numel(&res, C.int(i)))
		data := make([]float32, numel)
		if numel > 0 {
			C.tvm_out_copy(&res, C.int(i), (*C.float)(unsafe.Pointer(&data[0])))
		}
		outs[i] = Tensor{Shape: shape, Data: data}
	}
	return outs, nil
}
