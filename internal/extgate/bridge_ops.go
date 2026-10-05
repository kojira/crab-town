package extgate

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/kojira/crab-town/internal/world"
)

// Operations implements Handler: what the agent's actor can do in the world.
// Descriptions are built from the world (bounds, usable furniture) and say
// what each operation does -- never when or why to use it.
func (b *Bridge) Operations() []Operation {
	w, h := 0, 0
	var usable []*world.Furniture
	if a, ok := b.World.Actor(b.Actor); ok {
		for _, r := range b.World.Snapshot().Rooms {
			if r.ID != a.RoomID {
				continue
			}
			w, h = r.Width, r.Height
			for _, f := range r.Furniture {
				if h := r.HouseAt(f.Access); f.Function != "" && (h == nil || h.CanOperate(b.Actor)) {
					usable = append(usable, f)
				}
			}
		}
	}
	sort.Slice(usable, func(i, j int) bool { return usable[i].ID < usable[j].ID })

	ops := []Operation{{
		Name:  "look",
		Desc:  "今の自分から見える範囲の地図（現在地・ゾーン・ドア・家具・他のアクター）をテキストで返す。町は何も変わらない",
		Input: map[string]any{"type": "object", "properties": map[string]any{}},
		Run: func(map[string]any) (any, error) {
			return map[string]any{"map": b.View()}, nil
		},
		ReadOnly: true,
	}, {
		Name: "move",
		Desc: fmt.Sprintf("自分（%s）を町のタイル (x, y) まで歩かせる。座標は左上が (0,0)、x は 0〜%d、y は 0〜%d。"+
			"壁・家具のあるタイル、たどり着けないタイル、入る許可のない家の中は断られ、理由が返る。"+
			"受け付けると歩き始め、その様子は町の画面と Nostr の公開状態に出る", b.Actor, w-1, h-1),
		Input: map[string]any{
			"type":     "object",
			"required": []any{"x", "y"},
			"properties": map[string]any{
				"x": map[string]any{"type": "integer", "description": fmt.Sprintf("列（0〜%d）", w-1)},
				"y": map[string]any{"type": "integer", "description": fmt.Sprintf("行（0〜%d）", h-1)},
			},
		},
		Run: b.runMove,
	}}
	if len(usable) > 0 {
		ids := make([]any, 0, len(usable))
		var list []string
		for _, f := range usable {
			ids = append(ids, f.ID)
			list = append(list, fmt.Sprintf("%s=%s（%s）", f.ID, f.Label, f.Function))
		}
		ops = append(ops, Operation{
			Name: "interact",
			Desc: "自分を家具の前まで歩かせて、着いたらその家具を使う。入る許可のない家の家具は断られ、理由が返る。使える家具: " +
				strings.Join(list, "、"),
			Input: map[string]any{
				"type":     "object",
				"required": []any{"furniture"},
				"properties": map[string]any{
					"furniture": map[string]any{"type": "string", "enum": ids, "description": "家具の id"},
				},
			},
			Run: b.runInteract,
		})
	}
	return sortOps(ops)
}

// runMove: move payload {x, y} -> World.Move with by = the agent's own actor.
func (b *Bridge) runMove(payload map[string]any) (any, error) {
	x, err := intField(payload, "x")
	if err != nil {
		return nil, err
	}
	y, err := intField(payload, "y")
	if err != nil {
		return nil, err
	}
	to := world.Pos{X: x, Y: y}
	if err := b.World.Move(b.Actor, b.Actor, to); err != nil {
		return nil, worldReject(err, fmt.Sprintf("(%d,%d)", x, y))
	}
	return b.result("walking", map[string]any{"target": map[string]any{"x": x, "y": y}}), nil
}

// runInteract: interact payload {furniture} -> World.Interact, by = own actor.
func (b *Bridge) runInteract(payload map[string]any) (any, error) {
	id, ok := nonempty(payload, "furniture")
	if !ok {
		return nil, rejectf("furniture（家具の id）がない")
	}
	if err := b.World.Interact(b.Actor, b.Actor, id); err != nil {
		return nil, worldReject(err, id)
	}
	return b.result("interacting", map[string]any{"furniture": id}), nil
}

func (b *Bridge) result(status string, extra map[string]any) map[string]any {
	r := map[string]any{"status": status}
	for k, v := range extra {
		r[k] = v
	}
	if wa, ok := b.World.Where(b.Actor); ok {
		r["now"] = wa.String()
	}
	return r
}

// worldReject turns a world refusal into an operation_rejected reason.
func worldReject(err error, target string) error {
	switch {
	case errors.Is(err, world.ErrOutOfBounds):
		return rejectf("%s は町の範囲外", target)
	case errors.Is(err, world.ErrBlocked):
		return rejectf("%s は壁か家具で塞がっている", target)
	case errors.Is(err, world.ErrUnreachable):
		return rejectf("%s へ歩いて行ける経路がない", target)
	case errors.Is(err, world.ErrForbidden):
		return rejectf("%s へは入る許可がない（他人の家の中）", target)
	case errors.Is(err, world.ErrNoFurniture):
		return rejectf("%s という家具はない", target)
	case errors.Is(err, world.ErrNotUsable):
		return rejectf("%s は飾りで使えない", target)
	case errors.Is(err, world.ErrNoActor), errors.Is(err, world.ErrNoRoom):
		return rejectf("自分のアクターが町にいない")
	default:
		return rejectf("%s: %v", target, err)
	}
}
