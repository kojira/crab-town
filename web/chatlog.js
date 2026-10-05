// crab-town viewer: the chat log (talk / say / knock / system lines).
// entry() is pure (tested by node --test); the rest draws into #log.
const CrabChat = (() => {
  const MAX_LINES = 200;

  // Who said it, as the viewer sees it. ctx: { selfGuestId, selfActorId }.
  //   self     -- the viewer (their talk, or their own actor's say)
  //   resident -- a town actor speaking (say)
  //   owner    -- a talk verified as the town owner (not this viewer)
  //   guest    -- a visitor's talk
  //   system   -- knocks, command results, connection notes
  function entry(ev, ctx = {}) {
    if (!ev) return null;
    if (ev.type === "say" && ev.actor) {
      const id = ev.actor.id, name = ev.actor.name || id;
      const self = !!ctx.selfActorId && id === ctx.selfActorId;
      return { kind: self ? "self" : "resident", who: self ? `${name}（あなた）` : name, actor: id, text: ev.message || "" };
    }
    if (ev.type === "talk") {
      const self = !!ctx.selfGuestId && ev.by === ctx.selfGuestId;
      const kind = self ? "self" : ev.role === "owner" ? "owner" : "guest";
      const who = self ? "あなた" : kind === "owner" ? "オーナー" : "来客 " + String(ev.by || "").replace(/^nostr:/, "").slice(0, 8);
      return { kind, who, to: ev.to || "", text: ev.message || "" };
    }
    if (ev.type === "knock") return { kind: "system", who: "ノック", text: `${ev.by} → ${ev.house || ev.room} ${ev.message || ""}`.trim() };
    return null;
  }

  const doc = typeof document !== "undefined" ? document : null;
  const el = doc && doc.getElementById("log");
  function nearBottom() { return el.scrollHeight - el.scrollTop - el.clientHeight < 24; }

  // Append one line (newest at the bottom) and keep the newest in view unless
  // the reader scrolled up to read older lines.
  function add(e) {
    if (!e || !el) return;
    const stick = nearBottom();
    const row = document.createElement("div");
    row.className = "line " + e.kind + (e.actor ? " a-" + e.actor.replace(/[^a-z0-9_-]/gi, "") : "");
    const t = document.createElement("span");
    t.className = "t"; t.textContent = new Date().toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
    const w = document.createElement("span");
    w.className = "who"; w.textContent = e.who || "";
    const m = document.createElement("span");
    m.className = "msg"; m.textContent = (e.to ? "→ " + e.to + ": " : "") + (e.text || "");
    row.append(t, " ", w, " ", m);
    el.append(row);
    while (el.childElementCount > MAX_LINES) el.firstElementChild.remove();
    if (stick) el.scrollTop = el.scrollHeight;
    const n = doc.getElementById("chatCount");
    if (n) n.textContent = String(el.childElementCount);
  }
  function system(text) { add({ kind: "system", who: "", text }); }

  // Open / close (the toggle only shows on narrow screens).
  const toggle = doc && doc.getElementById("chatToggle");
  if (toggle) toggle.onclick = () => {
    const box = doc.getElementById("chat");
    const closed = box.classList.toggle("closed");
    toggle.setAttribute("aria-expanded", String(!closed));
    if (!closed) el.scrollTop = el.scrollHeight;
  };

  return { entry, add, system };
})();
if (typeof module !== "undefined") module.exports = CrabChat;
