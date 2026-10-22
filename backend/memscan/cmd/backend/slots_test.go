package main

import (
	"reflect"
	"sort"
	"strconv"
	"testing"

	"github.com/kayon/memscan/scanner"
)

// The slots are independent searches in the same game

func slotPositions(t *testing.T, a *App, slotIndex int, typ scanner.Type, addrs []uint64) (indexes []int) {
	t.Helper()
	results := a.scans[slotIndex].session.Scan(typ).Results()
	for _, addr := range addrs {
		i := sort.Search(len(results), func(i int) bool { return results[i] >= addr })
		if i < len(results) && results[i] == addr {
			indexes = append(indexes, i)
		}
	}
	if len(indexes) != len(addrs) {
		t.Fatalf("slot %d: only %d of %d planted %v are in the results", slotIndex, len(indexes), len(addrs), typ)
	}
	return
}

// plantedRows: the rows of the listed types that are in the area, by type name
func plantedRows(results *MultiResults, mem *area) map[string][][2]string {
	rows := make(map[string][][2]string)
	for _, tr := range results.Types {
		for _, row := range tr.List {
			addr, err := strconv.ParseUint(row[0], 16, 64)
			if err == nil && addr >= mem.start && addr < mem.start+uint64(len(mem.mem)) {
				rows[tr.Name] = append(rows[tr.Name], row)
			}
		}
	}
	return rows
}

func TestSlotsSearchSideBySide(t *testing.T) {
	a := newTestApp(t)
	areaA, areaB := newArea(t, 40*4096), newArea(t, 40*4096)
	numberA, numberB := randomNumber(), randomNumber()
	plantedA := plantEverywhere(t, areaA, numberA, 10)
	plantedB := plantEverywhere(t, areaB, numberB, 10)

	if a.FirstScanMulti(0, testAppID, numberA, scanner.OptionFloatUnrounded, false, 0) == nil {
		t.Fatal("first scan of slot 0 failed")
	}
	if a.FirstScanMulti(2, testAppID, numberB, scanner.OptionFloatUnrounded, false, 0) == nil {
		t.Fatal("first scan of slot 2 failed")
	}

	// each slot found its own number, and only that one
	for typ, want := range expectedHits(plantedA) {
		if got := areaA.within(a.scans[0].session.Scan(typ).Results()); !reflect.DeepEqual(got, want) {
			t.Fatalf("slot 0 %v: %d hits in its area, want %d", typ, len(got), len(want))
		}
		if got := areaB.within(a.scans[0].session.Scan(typ).Results()); len(got) != 0 {
			t.Fatalf("slot 0 %v holds %d results of the other slot", typ, len(got))
		}
	}
	for typ, want := range expectedHits(plantedB) {
		if got := areaB.within(a.scans[2].session.Scan(typ).Results()); !reflect.DeepEqual(got, want) {
			t.Fatalf("slot 2 %v: %d hits in its area, want %d", typ, len(got), len(want))
		}
	}

	// the slot in the middle never searched: nothing to continue
	if a.RefreshValuesMulti(1) != nil || a.NextScanMulti(1, "1") != nil || a.UndoScanMulti(1) != nil {
		t.Fatal("a slot without search must give nothing")
	}
}

func TestSlotScanDoesNotTouchTheOthers(t *testing.T) {
	a := newTestApp(t)
	areaA, areaB := newArea(t, 40*4096), newArea(t, 40*4096)
	numberA, numberB := randomNumber(), randomNumber()
	plantEverywhere(t, areaA, numberA, 10)
	plantEverywhere(t, areaB, numberB, 10)
	a.FirstScanMulti(0, testAppID, numberA, scanner.OptionFloatUnrounded, false, 0)
	a.FirstScanMulti(1, testAppID, numberB, scanner.OptionFloatUnrounded, false, 0)

	before := a.RefreshValuesMulti(1)
	if before == nil {
		t.Fatal("slot 1 has a search")
	}

	// slot 0 goes on: the value changes, then a next scan narrows it down
	changedA := randomNumber()
	plantEverywhere(t, areaA, changedA, 10)
	results := a.NextScanMulti(0, changedA)
	if results == nil || results.Round != 2 || !results.CanUndo {
		t.Fatalf("next scan of slot 0: %+v", results)
	}

	if rounds := a.scans[1].session.Rounds(); rounds != 1 {
		t.Fatalf("slot 1 is at round %d after a scan of slot 0", rounds)
	}
	after := a.RefreshValuesMulti(1)
	if after == nil || after.CanUndo || after.Round != before.Round {
		t.Fatalf("slot 1 changed:\n before %+v\n after  %+v", before, after)
	}
	// what the test planted is the same; the rest of the process
	if got, want := plantedRows(after, areaB), plantedRows(before, areaB); !reflect.DeepEqual(got, want) {
		t.Fatalf("slot 1 changed:\n before %v\n after  %v", want, got)
	}

	// undo of slot 0 does not undo slot 1, which has nothing to undo
	if a.UndoScanMulti(0) == nil {
		t.Fatal("slot 0 can undo")
	}
	if a.scans[1].session.CanUndo() {
		t.Fatal("slot 1 gained an undo")
	}
}

func TestSlotResetKeepsTheOthers(t *testing.T) {
	a := newTestApp(t)
	areaA, areaB := newArea(t, 40*4096), newArea(t, 40*4096)
	numberA, numberB := randomNumber(), randomNumber()
	plantEverywhere(t, areaA, numberA, 10)
	plantEverywhere(t, areaB, numberB, 10)
	a.FirstScanMulti(0, testAppID, numberA, scanner.OptionFloatUnrounded, false, 0)
	a.FirstScanMulti(1, testAppID, numberB, scanner.OptionFloatUnrounded, false, 0)

	a.ResetSlot(0)
	if a.RefreshValuesMulti(0) != nil {
		t.Fatal("slot 0 was reset")
	}
	if a.RefreshValuesMulti(1) == nil {
		t.Fatal("resetting slot 0 forgot slot 1")
	}

	// a slot that was reset can search again, with another number
	if a.FirstScanMulti(0, testAppID, numberB, scanner.OptionFloatUnrounded, false, 0) == nil {
		t.Fatal("slot 0 cannot search again")
	}

	// the game changed: everything is forgotten
	a.Clear()
	for i := 0; i < NumSlots; i++ {
		if a.RefreshValuesMulti(i) != nil {
			t.Fatalf("slot %d survived Clear", i)
		}
	}
}
