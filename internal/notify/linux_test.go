package notify

import (
	"reflect"
	"testing"
)

func TestNotifySendArgs(t *testing.T) {
	got := notifySendArgs(Request{Title: "T", Body: "B", Urgent: true})
	want := []string{"--app-name=Claude", "--urgency=critical", "--expire-time=10000", "T", "B"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
	if got := notifySendArgs(Request{Title: "T", Body: "B"}); got[1] != "--urgency=normal" {
		t.Fatalf("got %v", got)
	}
}
