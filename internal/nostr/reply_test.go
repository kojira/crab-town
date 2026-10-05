package nostr

import (
	"reflect"
	"testing"

	"github.com/kojira/crab-town/internal/world"
)

func TestReplyTagsOnlyForAnsweringSay(t *testing.T) {
	got := replyTags(world.Event{Type: world.EventSay, ReplyTo: "nostr:abcdef0123456789"})
	if want := [][]string{{"reply_to", "nostr:abcdef0123456789"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("tags = %v, want %v", got, want)
	}
	if got := replyTags(world.Event{Type: world.EventSay}); got != nil {
		t.Fatalf("say without reply_to: %v", got)
	}
	if got := replyTags(world.Event{Type: world.EventTalk, ReplyTo: "x"}); got != nil {
		t.Fatalf("talk: %v", got)
	}
}
