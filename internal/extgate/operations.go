package extgate

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
)

// Operation is one thing the agent may choose to do in crab-town. The same
// table produces the hello declaration (what core shows the agent as tools),
// the "取れる操作" list in said, and the invoke dispatch -- so what the agent is
// told it can do is exactly what is accepted.
type Operation struct {
	Name  string         // tool name (ASCII, unique)
	Desc  string         // shown to the agent as-is; describes, never instructs
	Input map[string]any // JSON Schema subset core accepts (type/required/properties/enum/items/description/format)
	// Run performs the operation for the agent's own actor. A non-nil error is
	// returned to core as operation_rejected with the error text as detail.
	Run func(payload map[string]any) (any, error)
}

// Declaration policy. dispatch=inline: core runs the call in the same turn and
// hands the result (or the rejection) straight back to the model, and the
// tool is in the always-projected set only via describe_tools otherwise
// (background is shown under "More tools" and awaited in a detached subtask).
const (
	opDispatch  = "inline"
	opSubEngine = "not_exposed"
	opSharing   = "agent_bound"
	opEffect    = "state_change"
)

// opCallers: core checks the turn's caller class against this list. crab-town
// does its own check on the actor (by = the agent's actor), so every class may
// let the agent act. Must be sorted ascending.
var opCallers = []any{"co_agent", "guest", "owner", "trusted"}

// ErrRejected wraps every refusal so callers can tell it from a wire problem.
var ErrRejected = errors.New("operation_rejected")

// sortOps returns ops sorted by name (core requires ascending, no duplicates).
func sortOps(ops []Operation) []Operation {
	out := append([]Operation(nil), ops...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Declarations renders ops as hello's operations array.
func Declarations(ops []Operation) []any {
	out := []any{}
	for _, op := range sortOps(ops) {
		out = append(out, map[string]any{
			"name":            op.Name,
			"description":     op.Desc,
			"input_schema":    op.Input,
			"output_schema":   nil,
			"callback_schema": nil,
			"authorization":   map[string]any{"allowed_callers": opCallers},
			"dispatch":        opDispatch,
			"sub_engine":      opSubEngine,
			"sharing":         opSharing,
			"effect":          opEffect,
		})
	}
	return out
}

// Listing renders ops for a said text: one "- name: desc（payload: {...}）" line each.
func Listing(ops []Operation) string {
	var sb strings.Builder
	for _, op := range sortOps(ops) {
		sb.WriteString("\n- " + op.Name + ": " + op.Desc)
		if p := payloadShape(op.Input); p != "" {
			sb.WriteString("（payload: " + p + "）")
		}
	}
	return sb.String()
}

// payloadShape: {"x": integer, "y": integer} from the top-level properties.
func payloadShape(schema map[string]any) string {
	props, _ := schema["properties"].(map[string]any)
	if len(props) == 0 {
		return ""
	}
	names := make([]string, 0, len(props))
	for n := range props {
		names = append(names, n)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, n := range names {
		t := "any"
		if p, ok := props[n].(map[string]any); ok {
			if s, ok := p["type"].(string); ok {
				t = s
			}
		}
		parts = append(parts, fmt.Sprintf("%q: %s", n, t))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func rejectf(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrRejected, fmt.Sprintf(format, a...))
}

// intField reads an integer payload field (json.Number from the decoder, or a
// float64 holding an integer).
func intField(payload map[string]any, key string) (int, error) {
	switch v := payload[key].(type) {
	case json.Number:
		n, err := v.Int64()
		if err != nil || n < math.MinInt32 || n > math.MaxInt32 {
			return 0, rejectf("%s は整数で指定する（受け取った値: %s）", key, v)
		}
		return int(n), nil
	case float64:
		if v != math.Trunc(v) || math.Abs(v) > math.MaxInt32 {
			return 0, rejectf("%s は整数で指定する（受け取った値: %v）", key, v)
		}
		return int(v), nil
	case nil:
		return 0, rejectf("%s がない", key)
	default:
		return 0, rejectf("%s は整数で指定する（受け取った値: %v）", key, v)
	}
}
