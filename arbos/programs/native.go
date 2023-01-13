// Copyright 2022-2023, Offchain Labs, Inc.
// For license information, see https://github.com/nitro/blob/master/LICENSE

//go:build !js
// +build !js

package programs

/*
#cgo CFLAGS: -g -Wall
#cgo LDFLAGS: ${SRCDIR}/../../target/lib/libstylus.a -ldl -lm
#include <stdint.h>

typedef struct GoParams {
  uint32_t version;
  uint32_t max_depth;
  uint32_t max_frame_size;
  uint32_t heap_bound;
  uint64_t wasm_gas_price;
  uint64_t hostio_cost;
} GoParams;

typedef struct GoSlice {
  const uint8_t * ptr;
  const size_t len;
} GoSlice;

typedef struct RustVec {
  uint8_t * const * ptr;
  size_t * len;
  size_t * cap;
} RustVec;

extern uint8_t stylus_compile(GoSlice wasm, GoParams params, RustVec output);
extern uint8_t stylus_call(GoSlice module, GoSlice calldata, GoParams params, RustVec output, uint64_t * evm_gas);
extern void    stylus_free(RustVec vec);
*/
import "C"
import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"
	"github.com/offchainlabs/nitro/arbutil"
)

type u8 = C.uint8_t
type u32 = C.uint32_t
type u64 = C.uint64_t
type usize = C.size_t

func compileUserWasm(db vm.StateDB, program common.Address, wasm []byte, params *GoParams) error {
	output := rustVec()
	status := userStatus(C.stylus_compile(
		goSlice(wasm),
		params.encode(),
		output,
	))
	result, err := status.output(output.read())
	if err == nil {
		db.AddUserModule(params.Version, program, result)
	}
	return err
}

func callUserWasm(db vm.StateDB, program common.Address, calldata []byte, gas *uint64, params *GoParams) ([]byte, error) {

	if db, ok := db.(*state.StateDB); ok {
		db.RecordProgram(program)
	}
	if db.Deterministic() {
		_ = db.GetCode(program) // mirror the state access in wasm.go to collect the preimage(s)
	}

	module, err := db.GetUserModule(1, program)
	if err != nil {
		log.Crit("instance module does not exist")
	}

	output := rustVec()
	status := userStatus(C.stylus_call(
		goSlice(module),
		goSlice(calldata),
		params.encode(),
		output,
		(*u64)(gas),
	))
	return status.output(output.read())
}

func rustVec() C.RustVec {
	var ptr *u8
	var len usize
	var cap usize
	return C.RustVec{
		ptr: (**u8)(&ptr),
		len: (*usize)(&len),
		cap: (*usize)(&cap),
	}
}

func (vec C.RustVec) read() []byte {
	slice := arbutil.PointerToSlice((*byte)(*vec.ptr), int(*vec.len))
	C.stylus_free(vec)
	return slice
}

func goSlice(slice []byte) C.GoSlice {
	return C.GoSlice{
		ptr: (*u8)(arbutil.SliceToPointer(slice)),
		len: usize(len(slice)),
	}
}

func (params *GoParams) encode() C.GoParams {
	return C.GoParams{
		version:        u32(params.Version),
		max_depth:      u32(params.MaxDepth),
		max_frame_size: u32(params.MaxFrameSize),
		heap_bound:     u32(params.HeapBound),
		wasm_gas_price: u64(params.WasmGasPrice),
		hostio_cost:    u64(params.HostioCost),
	}
}
