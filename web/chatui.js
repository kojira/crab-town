// crab-town viewer: chat state that is easy to get wrong, as pure functions
// (tested by node --test): sending / delivered / failed talks, the history
// replayed after a reload, which result lines are worth showing, replies
// addressed to the viewer, and the speech bubble's wrapping and lifetime.
const CrabTalk = (() => {
  // ---- sending -------------------------------------------------------------
  // A talk the viewer typed: shown at once as "送信中", then "届いた" when the
  // town answers ok (or echoes the talk), "送れなかった" otherwise.
  const STATUS_TEXT = { sending: "送信中…", delivered: "届いた", failed: "送れなかった" };
  function outbox() {
    const items = []; let seq = 0;
    return {
      items,
      add(text) { const it = { key: ++seq, text, status: "sending", eventId: "", error: "" }; items.push(it); return it; },
      // after publishing: n relays took the event. 0 = nothing left the page.
      sent(it, eventId, n) {
        it.eventId = eventId || "";
        if (!(n > 0)) { it.status = "failed"; it.error = "つながっているリレーが無い（0本）"; }
        return it;
      },
      signFailed(it, err) { it.status = "failed"; it.error = String(err || "署名できなかった"); return it; },
      // the town's result for one of our events (matched by its e tag)
      result(eventId, ok, error) {
        const it = items.find(x => x.eventId && x.eventId === eventId);
        if (!it || it.status === "delivered") return null; // already confirmed (echo)
        if (ok) it.status = "delivered"; else { it.status = "failed"; it.error = error || "町が受け付けなかった"; }
        return it;
      },
      // the town echoing a talk event by us with this text: it arrived
      echo(text) {
        const it = items.find(x => x.text === text && x.status !== "failed" && !x.echoed);
        if (!it) return null;
        it.echoed = true; it.status = "delivered";
        return it;
      },
      remove(it) { const i = items.indexOf(it); if (i >= 0) items.splice(i, 1); },
    };
  }
  // Whether the input may be cleared: only once the talk is known to be in.
  const mayClear = (it) => !!it && it.status === "delivered";

  // ---- history -------------------------------------------------------------
  // Relays replay stored events (often newest first) until EOSE. Buffer them
  // and apply oldest first; ties keep a natural order (talk before result before say).
  const RANK = { snapshot: 0, actor: 1, occupancy: 1, knock: 2, talk: 2, result: 3, say: 4 };
  function sortBacklog(list) {
    return list.map((x, i) => ({ x, i })).sort((a, b) =>
      (a.x.created_at - b.x.created_at) || ((RANK[a.x.type] ?? 1) - (RANK[b.x.type] ?? 1)) || (a.i - b.i)).map(o => o.x);
  }

  // ---- result lines --------------------------------------------------------
  // ok results are shown in the status of the line they belong to, not as
  // lines of their own; only refusals get a line.
  const resultWorthALine = (ev) => !!ev && ev.type === "result" && !ev.ok;

  // ---- replies -------------------------------------------------------------
  // A say carries reply_to (the talker it answers). For the viewer it is
  // "→ あなた", someone else's exchange (dimmed), or said to nobody.
  function addressed(ev, selfId) {
    if (!ev || !ev.reply_to) return "none";
    return selfId && ev.reply_to === selfId ? "you" : "other";
  }

  // ---- speech bubbles ------------------------------------------------------
  // Wrap into lines no wider than maxW (measure: text -> px). Breaks between
  // characters (Japanese has no spaces); at most maxLines, the last ends in "…".
  function wrap(text, maxW, measure, maxLines = Infinity) {
    const out = [];
    for (const para of String(text || "").split("\n")) {
      let line = "";
      for (const ch of para) {
        if (line && measure(line + ch) > maxW) { out.push(line); line = ""; }
        line += ch;
      }
      out.push(line);
    }
    if (out.length > maxLines) { out.length = maxLines; out[maxLines - 1] = out[maxLines - 1].slice(0, -1) + "…"; }
    return out;
  }
  // How long a bubble stays: long enough to read (about 8 characters a second).
  const SPEECH_MIN_MS = 6000, SPEECH_MAX_MS = 60000;
  function speechMs(text) {
    const n = [...String(text || "")].length;
    return Math.max(SPEECH_MIN_MS, Math.min(SPEECH_MAX_MS, 3000 + n * 125));
  }

  // ---- input hint ----------------------------------------------------------
  const HINT_LOGIN = "NIP-07 拡張（nos2x・Alby など）でログインすると話せる";
  const HINT_TALK = "のすたろうに話しかける（平文・公開・280字まで）";
  const inputHint = (loggedIn) => (loggedIn ? HINT_TALK : HINT_LOGIN);

  return { STATUS_TEXT, outbox, mayClear, sortBacklog, resultWorthALine, addressed, wrap, speechMs,
    SPEECH_MIN_MS, SPEECH_MAX_MS, inputHint, HINT_LOGIN };
})();
if (typeof module !== "undefined") module.exports = CrabTalk;
