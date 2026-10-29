package winproc

import (
	"reflect"
	"testing"
)

func TestAncestors(t *testing.T) {
	chain := map[uint32]Proc{4: {Parent: 3}, 3: {Parent: 2}, 2: {Parent: 1}, 1: {}}
	if got := Ancestors(4, chain); !reflect.DeepEqual(got, []uint32{3, 2, 1}) {
		t.Fatalf("chain: %v", got)
	}
	loop := map[uint32]Proc{2: {Parent: 3}, 3: {Parent: 2}}
	if got := Ancestors(2, loop); !reflect.DeepEqual(got, []uint32{3, 2}) {
		t.Fatalf("loop: %v", got)
	}
	if got := Ancestors(9, chain); len(got) != 0 {
		t.Fatalf("unknown pid: %v", got)
	}
}
