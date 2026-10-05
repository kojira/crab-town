// crab-town viewer: the chat log (talk / say / knock / system lines).
// entry() is pure (tested by node --test); the rest draws into #log.
const CrabChat = (() => {
  const MAX_LINES = 200;

  // Who said it, as the viewer sees it. ctx: { selfGuestId, houses, listening }.
  //   self     -- the viewer's own talk (the logged-in pubkey, owner or guest)
  //   resident -- a town actor speaking (say). Never "you": even when the
  //               owner drives that actor, its words come from the agent.
  //   owner    -- a talk verified as the town owner (not this viewer)
  //   guest    -- a visitor's talk
  //   system   -- knocks, command results, connection notes
  function entry(ev, ctx = {}) {
    if (!ev) return null;
    const at = ev.at || 0; // event time (created_at, seconds); 0 = now
    if (ev.type === "say" && ev.actor) {
      const id = ev.actor.id;
      // reply_to: the talker this answers -- "→ あなた", or dimmed when it is someone else's
      const you = !!ctx.selfGuestId && ev.reply_to === ctx.selfGuestId;
      const other = !!ev.reply_to && !you;
      return { kind: "resident", who: ev.actor.name || id, actor: id, text: ev.message || "", at,
        to: you ? "あなた" : other && ctx.selfGuestId ? "来客 " + short(ev.reply_to) : "", dim: other && !!ctx.selfGuestId };
    }
    if (ev.type === "talk") {
      const self = !!ctx.selfGuestId && ev.by === ctx.selfGuestId;
      const kind = self ? "self" : ev.role === "owner" ? "owner" : "guest";
      const who = self ? "あなた" : kind === "owner" ? "オーナー" : "来客 " + short(ev.by);
      // someone else's exchange is dimmed for a logged-in viewer
      return { kind, who, to: ev.to || "", text: ev.message || "", at, dim: !self && !!ctx.selfGuestId };
    }
    if (ev.type === "knock") {
      const h = (ctx.houses || []).find(x => x.id === ev.house);
      const by = ctx.selfGuestId && ev.by === ctx.selfGuestId ? "あなた" : String(ev.by || "").startsWith("nostr:") ? "来客 " + short(ev.by) : ev.by;
      let text = `${by} → ${h ? h.label : ev.house || ev.room} ${ev.message || ""}`.trim();
      if (h && !h.reachable) text += `（${h.ownerName}は今つながっていないので届かない。町の出来事として記録だけ）`;
      return { kind: "system", who: "ノック", text, at };
    }
    return null;
  }
  function short(by) { return String(by || "").replace(/^nostr:/, "").slice(0, 8); }

  // A command result as a readable line (the town answers with error strings).
  function resultText(ev) {
    if (ev.ok) return `${ev.cmd}: ok (${ev.role})`;
    const why = /forbidden/.test(ev.error || "") ? (ev.cmd === "move" ? "その場所には入れない（招待されていない家・見えない部屋）" : "権限がない") : ev.error;
    return `${ev.cmd}: NG ${why} (${ev.role})`;
  }

  // How far the on-screen keyboard covers the bottom of the layout viewport
  // (px), from window.innerHeight and window.visualViewport. 0 without one.
  function keyboardInset(innerH, vv) {
    if (!vv || !(innerH > 0)) return 0;
    const d = innerH - (vv.height + (vv.offsetTop || 0));
    return d > 1 ? Math.round(d) : 0;
  }

  const doc = typeof document !== "undefined" ? document : null;
  const el = doc && doc.getElementById("log");
  function nearBottom() { return el.scrollHeight - el.scrollTop - el.clientHeight < 24; }
  // "↓ 新着": a line arrived while the reader was scrolled up
  const newBtn = doc && doc.getElementById("newLines");
  const showNew = (on) => { if (newBtn) newBtn.hidden = !on; };
  if (newBtn) newBtn.onclick = () => { el.scrollTop = el.scrollHeight; showNew(false); };
  if (el) el.addEventListener("scroll", () => { if (nearBottom()) showNew(false); });
  const hhmm = (at) => (at ? new Date(at * 1000) : new Date()).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });

  // Append one line (newest at the bottom) and keep the newest in view unless
  // the reader scrolled up to read older lines.
  function add(e) {
    if (!e || !el) return;
    const stick = nearBottom();
    const row = document.createElement("div");
    row.className = "line " + e.kind + (e.actor ? " a-" + e.actor.replace(/[^a-z0-9_-]/gi, "") : "") + (e.dim ? " dim" : "") + (e.error ? " err" : "");
    const t = document.createElement("span");
    t.className = "t"; t.textContent = hhmm(e.at);
    const w = document.createElement("span");
    w.className = "who"; w.textContent = e.who || "";
    const m = document.createElement("span");
    m.className = "msg"; m.textContent = (e.to ? "→ " + e.to + ": " : "") + (e.text || "");
    row.append(t, " ", w, " ", m);
    el.append(row);
    while (el.childElementCount > MAX_LINES) el.firstElementChild.remove();
    if (stick) el.scrollTop = el.scrollHeight; else showNew(true);
    const n = doc.getElementById("chatCount");
    if (n) n.textContent = String(el.childElementCount);
    return row;
  }
  function system(text) { return add({ kind: "system", who: "", text }); }
  function error(text) { return add({ kind: "system", who: "", text, error: true }); }

  // The delivery status of the viewer's own talk line: 送信中… / 届いた / 送れなかった
  // (red, with the reason and a 再送 button).
  function setStatus(row, status, label, why, retry) {
    if (!row) return;
    let st = row.querySelector(".st");
    if (!st) { st = document.createElement("span"); row.append(" ", st); }
    st.className = "st " + status;
    st.textContent = status === "failed" ? `${label}: ${why}` : label;
    if (status === "failed" && retry) {
      const b = document.createElement("button");
      b.type = "button"; b.className = "retry"; b.textContent = "再送"; b.onclick = retry;
      st.append(" ", b);
    }
  }
  // "のすたろうが入力中…" under the log while a reply is being written.
  function typing(name) {
    const t = doc && doc.getElementById("typing");
    if (!t) return;
    t.hidden = !name;
    t.textContent = name ? name + "が入力中…" : "";
    if (name && nearBottom()) el.scrollTop = el.scrollHeight;
  }

  // Open / close (the toggle only shows on narrow screens); it names what it does.
  const toggleLabel = (closed) => (closed ? "▲ ログを開く" : "▼ ログを閉じる");
  const toggle = doc && doc.getElementById("chatToggle");
  if (toggle) toggle.onclick = () => {
    const box = doc.getElementById("chat");
    const closed = box.classList.toggle("closed");
    toggle.setAttribute("aria-expanded", String(!closed));
    toggle.textContent = toggleLabel(closed);
    if (!closed) el.scrollTop = el.scrollHeight;
    if (typeof CrabView !== "undefined") CrabView.layout();
  };

  // Phones: keep the fixed chat panel (and its input) above the soft keyboard.
  const vv = typeof window !== "undefined" && window.visualViewport;
  const box = doc && doc.getElementById("chat");
  function lift() {
    const k = doc.body.classList.contains("narrow") ? keyboardInset(window.innerHeight, vv) : 0;
    box.style.bottom = k ? k + "px" : "";
    if (vv) doc.body.style.setProperty("--vv-h", Math.round(vv.height) + "px");
  }
  // Typing on a phone: give the log more room while the input has focus.
  const input = doc && doc.getElementById("talkText");
  if (input && box) {
    input.addEventListener("focus", () => { box.classList.add("focus"); box.classList.remove("closed"); if (toggle) { toggle.textContent = toggleLabel(false); toggle.setAttribute("aria-expanded", "true"); } el.scrollTop = el.scrollHeight; lift(); });
    input.addEventListener("blur", () => { setTimeout(() => box.classList.remove("focus"), 150); });
  }
  if (vv && box) { vv.addEventListener("resize", lift); vv.addEventListener("scroll", lift); }

  // History across tabs: state events are ephemeral (kind 23411), relays keep
  // them for minutes at most, so the browser keeps the last talk/say/knock lines.
  const HIST_KEY = "crab-town-chat-v1", HIST_MAX = 200;
  const HIST_TYPES = { talk: 1, say: 1, knock: 1 };
  function keep(list, item, max = HIST_MAX) {
    if (!item || !item.id || !item.msg || !HIST_TYPES[item.msg.type] || list.some(x => x.id === item.id)) return list;
    const out = list.concat([item]);
    return out.length > max ? out.slice(out.length - max) : out;
  }
  function loadHistory(store) {
    try { const v = JSON.parse((store && store.getItem(HIST_KEY)) || "[]"); return Array.isArray(v) ? v.filter(x => x && x.id && x.msg && HIST_TYPES[x.msg.type]) : []; }
    catch { return []; }
  }
  function saveHistory(store, list) { try { store && store.setItem(HIST_KEY, JSON.stringify(list)); } catch { /* full / private mode */ } }

  return { entry, resultText, add, system, error, setStatus, typing, keyboardInset, toggleLabel, keep, loadHistory, saveHistory, HIST_KEY };
})();
if (typeof module !== "undefined") module.exports = CrabChat;
