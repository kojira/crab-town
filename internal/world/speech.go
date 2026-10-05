package world

// EventSay carries an utterance shown as a speech bubble over the actor.
const EventSay = "say"

// MaxSpeech caps the bubble text (runes); the rest is cut with an ellipsis.
const MaxSpeech = 140

// Speak emits a say event for the actor. It does not change the actor's state
// and nothing leaves crab-town: it is display only.
func (w *World) Speak(actorID, text string) error {
	if text == "" {
		return ErrBadRequest
	}
	if r := []rune(text); len(r) > MaxSpeech {
		text = string(r[:MaxSpeech]) + "…"
	}
	w.mu.Lock()
	a, r, err := w.lookup(actorID)
	if err != nil {
		w.mu.Unlock()
		return err
	}
	ev := Event{Type: EventSay, Room: r.ID, Actor: copyActor(a), By: a.ID, Message: text}
	w.mu.Unlock()
	w.emit(ev)
	return nil
}
