package tray

import (
	"testing"

	"github.com/sundeiii/yeet-client/pkg/puush"
)

func TestSameHistory(t *testing.T) {
	a := []*puush.HistoryItem{{Id: 1, FileName: "a.png", Views: 2}, {Id: 2, FileName: "b.png"}}
	same := []*puush.HistoryItem{{Id: 1, FileName: "a.png", Views: 2}, {Id: 2, FileName: "b.png"}}

	if !sameHistory(a, same) {
		t.Error("identical lists should count as the same")
	}
	if sameHistory(a, a[:1]) {
		t.Error("a shorter list is a change")
	}
	if sameHistory(a, []*puush.HistoryItem{{Id: 3, FileName: "c.png"}, {Id: 1, FileName: "a.png", Views: 2}}) {
		t.Error("a new upload is a change")
	}
	if sameHistory(a, []*puush.HistoryItem{{Id: 1, FileName: "a.png", Views: 3}, {Id: 2, FileName: "b.png"}}) {
		t.Error("a new view is a change")
	}
	if !sameHistory(nil, []*puush.HistoryItem{}) {
		t.Error("two empty lists are the same")
	}
}
