package next

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/kojira/crab-town/internal/world"
)

// The houses and residents are data: an approval survives a restart, and the
// committed example equals what `town-next export` makes of the current town.
func TestApprovalPersists(t *testing.T) {
	_, owner := newKey(t)
	_, pk := newKey(t)
	path := filepath.Join(t.TempDir(), "town.json")
	if err := testData(owner, "").Save(path); err != nil {
		t.Fatal(err)
	}
	d, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	tw, err := New(d, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := tw.Apply(pk, "plot"); err != nil {
		t.Fatal(err)
	}
	if err := tw.Approve(owner, "plot", pk); err != nil {
		t.Fatal(err)
	}
	d2, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	tw2, _ := New(d2, path)
	if r := tw2.RightsOf(pk); !reflect.DeepEqual(r.Holds, []string{"plot"}) {
		t.Fatalf("after reload: %+v", r)
	}
	if len(d2.Applications) != 0 {
		t.Fatalf("applications left: %+v", d2.Applications)
	}
}

func TestValidateRejects(t *testing.T) {
	_, owner := newKey(t)
	bad := map[string]func(d *Data){
		"owner":       func(d *Data) { d.Owner = "kojira" },
		"holder name": func(d *Data) { d.Rooms[0].Houses[0].Owner = "nostarou" },
		"spawn":       func(d *Data) { d.Spawn.Pos = world.Pos{X: 99, Y: 0} },
		"dup house":   func(d *Data) { d.Rooms[0].Houses[1].ID = "a-house" },
	}
	for name, mut := range bad {
		d := testData(owner, "")
		mut(d)
		if d.Validate() == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestExampleMatchesExport(t *testing.T) {
	b, err := os.ReadFile("../../town-next.example.json")
	if err != nil {
		t.Fatal(err)
	}
	var ex Data
	if err := json.Unmarshal(b, &ex); err != nil {
		t.Fatal(err)
	}
	if err := ex.Validate(); err != nil {
		t.Fatal(err)
	}
	holders := map[string]string{}
	for _, n := range ex.Names {
		if n.Name == "のすたろう" {
			holders["nostarou"] = n.Pubkey
		}
	}
	got := FromWorld(world.NewDefault(), ex.Owner, ex.Spawn, holders)
	gb, _ := json.Marshal(got)
	eb, _ := json.Marshal(&ex)
	if string(gb) != string(eb) {
		t.Fatal("town-next.example.json is out of date: regenerate with `go run ./cmd/town-next export`")
	}
}
