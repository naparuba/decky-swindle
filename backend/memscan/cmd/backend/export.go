// Copyright (C) 2026 naparuba <naparuba@gmail.com>

package main

/*
#include <stdint.h>
#include <stdlib.h>
*/
import "C"
import (
	"encoding/json"
	"unsafe"

	"github.com/kayon/memscan/scanner"
)

func returnJSON(v interface{}) *C.char {
	data, _ := json.Marshal(v)
	return C.CString(string(data))
}

// indexes converts the positions given by the caller (a C array)
func indexes(cIndexes *C.int32_t, length C.int) []int {
	idxSlice := unsafe.Slice((*int32)(cIndexes), int(length))
	converted := make([]int, len(idxSlice))
	for i, v := range idxSlice {
		converted[i] = int(v)
	}
	return converted
}

//export FreeString
func FreeString(p *C.char) {
	C.free(unsafe.Pointer(p))
}

// DebugDumpStacks returns the stack of every goroutine of the engine
//export DebugDumpStacks
func DebugDumpStacks() *C.char {
	return C.CString(dumpStacks())
}

//export SetRenderResultsThreshold
func SetRenderResultsThreshold(value C.int) {
	app.mu.Lock()
	defer app.mu.Unlock()
	app.SetRenderResultsThreshold(int(value))
}

//export Version
func Version() *C.char {
	return returnJSON(version)
}

//export Clear
func Clear() {
	app.mu.Lock()
	defer app.mu.Unlock()
	app.Clear()
}

//export ResetScan
func ResetScan() {
	app.mu.Lock()
	defer app.mu.Unlock()
	app.ResetScan()
}

// ResetSlot forgets the search of one slot; the others stay as they are
//export ResetSlot
func ResetSlot(slot C.int) {
	app.mu.Lock()
	defer app.mu.Unlock()
	app.ResetSlot(int(slot))
}

//export GetGameProcesses
func GetGameProcesses(appID C.int64_t) *C.char {
	app.mu.Lock()
	defer app.mu.Unlock()
	processes := app.GetGameProcesses(int64(appID))
	return returnJSON(processes)
}

//export SelectGameProcess
func SelectGameProcess(appID C.int64_t, pid C.int) *C.char {
	app.mu.Lock()
	defer app.mu.Unlock()
	process := app.SelectGameProcess(int64(appID), int(pid))
	return returnJSON(process)
}

//export AutoSelectGameProcess
func AutoSelectGameProcess(appID C.int64_t) *C.char {
	app.mu.Lock()
	defer app.mu.Unlock()
	process := app.AutoSelectGameProcess(int64(appID))
	return returnJSON(process)
}


// FirstScanMulti: typesMask selects the types to search, one bit per type number
//export FirstScanMulti
func FirstScanMulti(slot C.int, appID C.int64_t, value *C.char, option C.int, emulator C.int, typesMask C.int) *C.char {
	app.mu.Lock()
	defer app.mu.Unlock()
	results := app.FirstScanMulti(int(slot), int64(appID), C.GoString(value), scanner.Option(option), emulator != 0, uint(typesMask))
	return returnJSON(results)
}

//export NextScanMulti
func NextScanMulti(slot C.int, value *C.char) *C.char {
	app.mu.Lock()
	defer app.mu.Unlock()
	results := app.NextScanMulti(int(slot), C.GoString(value))
	return returnJSON(results)
}

//export UndoScanMulti
func UndoScanMulti(slot C.int) *C.char {
	app.mu.Lock()
	defer app.mu.Unlock()
	results := app.UndoScanMulti(int(slot))
	return returnJSON(results)
}

// KeepOnlyMulti drops the types that are not in typesMask
//export KeepOnlyMulti
func KeepOnlyMulti(slot C.int, typesMask C.int) *C.char {
	app.mu.Lock()
	defer app.mu.Unlock()
	results := app.KeepOnlyMulti(int(slot), uint(typesMask))
	return returnJSON(results)
}

// ChangeValuesMulti writes value over the results of one type (a type number)
//export ChangeValuesMulti
func ChangeValuesMulti(slot C.int, valueType C.int, value *C.char, cIndexes *C.int32_t, length C.int) *C.char {
	app.mu.Lock()
	defer app.mu.Unlock()
	results := app.ChangeValuesMulti(int(slot), scanner.Type(valueType), C.GoString(value), indexes(cIndexes, length))
	return returnJSON(results)
}

//export RefreshValuesMulti
func RefreshValuesMulti(slot C.int) *C.char {
	app.mu.Lock()
	defer app.mu.Unlock()
	results := app.RefreshValuesMulti(int(slot))
	return returnJSON(results)
}


//export FirstScan
func FirstScan(appID C.int64_t, value *C.char, valueType C.int, option C.int, emulator C.int) *C.char {
	app.mu.Lock()
	defer app.mu.Unlock()
	results := app.FirstScan(int64(appID), C.GoString(value), scanner.Type(valueType), scanner.Option(option), emulator != 0)
	return returnJSON(results)
}

//export NextScan
func NextScan(value *C.char) *C.char {
	app.mu.Lock()
	defer app.mu.Unlock()
	results := app.NextScan(C.GoString(value))
	return returnJSON(results)
}

//export UndoScan
func UndoScan() *C.char {
	app.mu.Lock()
	defer app.mu.Unlock()
	results := app.UndoScan()
	return returnJSON(results)
}

//export ChangeValues
func ChangeValues(value *C.char, cIndexes *C.int32_t, length C.int) *C.char {
	app.mu.Lock()
	defer app.mu.Unlock()
	results := app.ChangeValues(C.GoString(value), indexes(cIndexes, length))
	return returnJSON(results)
}

//export RefreshValues
func RefreshValues() *C.char {
	app.mu.Lock()
	defer app.mu.Unlock()
	results := app.RefreshValues()
	return returnJSON(results)
}
