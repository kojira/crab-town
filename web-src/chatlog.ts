// crab-town viewer: the chat log (talk / say / knock / system lines).
// entry() is pure (tested by node --test); the rest draws into #log.
const CrabChat = (() => {
  const MAX_LINES = 200;
  // a house as the knock menu knows it
  interface HouseInfo { id: string; label: string; reachable?: boolean; ownerName?: string }
  interface EntryCtx { selfGuestId?: string | null; houses?: HouseInfo[]; listening?: string[] }
  // one line of the log
  interface Entry { kind: string; who: string; text: string; at?: number; to?: string; actor?: string; dim?: boolean; error?: boolean; image?: string }

  // Who said it, as the viewer sees it. ctx: { selfGuestId, houses, listening }.
  //   self     -- the viewer's own talk (the logged-in pubkey, owner or guest)
  //   resident -- a town actor speaking (say). Never "you": even when the
  //               owner drives that actor, its words come from the agent.
  //   owner    -- a talk verified as the town owner (not this viewer)
  //   guest    -- a visitor's talk
  //   system   -- knocks, command results, connection notes
  function entry(ev: WorldMsg | null | undefined, ctx: EntryCtx = {}): Entry | null {
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
      // image: only an owner talk carries one (the town refuses others); drawn only if https
      const image = ev.role === "owner" && ev.image ? ev.image : undefined;
      return { kind, who, to: ev.to || "", text: ev.message || "", at, dim: !self && !!ctx.selfGuestId, image };
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
  function short(by: string | undefined) { return String(by || "").replace(/^nostr:/, "").slice(0, 8); }

  // A command result as a readable line (the town answers with error strings).
  function resultText(ev: ResultMsg) {
    if (ev.ok) return `${ev.cmd}: ok (${ev.role})`;
    const why = /forbidden/.test(ev.error || "") ? (ev.cmd === "move" ? "その場所には入れない（招待されていない家・見えない部屋）" : "権限がない") : ev.error;
    return `${ev.cmd}: NG ${why} (${ev.role})`;
  }

  // How far the on-screen keyboard covers the bottom of the layout viewport
  // (px), from window.innerHeight and window.visualViewport. 0 without one.
  function keyboardInset(innerH: number, vv: { height: number; offsetTop?: number } | null | undefined | false) {
    if (!vv || !(innerH > 0)) return 0;
    const d = innerH - (vv.height + (vv.offsetTop || 0));
    return d > 1 ? Math.round(d) : 0;
  }

  // The part of the page the reader actually sees: the visual viewport when
  // the browser has one (it shrinks above the soft keyboard), else the window.
  function appViewport(innerH: number, vv: { height: number; offsetTop?: number } | null | undefined | false) {
    if (!vv || !(vv.height > 0)) return { height: Math.round(innerH), top: 0 };
    return { height: Math.round(Math.min(vv.height, innerH > 0 ? innerH : vv.height)), top: Math.max(0, Math.round(vv.offsetTop || 0)) };
  }

  const doc = typeof document !== "undefined" ? document : null;
  const el = doc && doc.getElementById("log");
  // el / doc are null only under node --test, where nothing below is called
  function nearBottom() { const el = logBox(); return el.scrollHeight - el.scrollTop - el.clientHeight < 24; }
  // "↓ 新着": a line arrived while the reader was scrolled up
  const newBtn = doc && doc.getElementById("newLines");
  const showNew = (on: boolean) => { if (newBtn) newBtn.hidden = !on; };
  if (newBtn) newBtn.onclick = () => { const el = logBox(); el.scrollTop = el.scrollHeight; showNew(false); };
  // the reader was at the newest line: keep them there when the log changes size (keyboard, header folding)
  let atEnd = true;
  if (el) el.addEventListener("scroll", () => { atEnd = nearBottom(); if (atEnd) showNew(false); });
  if (el && typeof ResizeObserver !== "undefined") new ResizeObserver(() => { if (atEnd) el.scrollTop = el.scrollHeight; }).observe(el);
  const hhmm = (at: number | undefined) => (at ? new Date(at * 1000) : new Date()).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });

  // Append one line (newest at the bottom) and keep the newest in view unless
  // the reader scrolled up to read older lines.
  function logBox(): HTMLElement { return el!; }
  function add(e: Entry | null | undefined): HTMLDivElement | undefined {
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
    // an image: <a><img></a> built from DOM properties (CrabUpload.imageNode), never innerHTML
    const pic = e.image && typeof CrabUpload !== "undefined" ? CrabUpload.imageNode(document, e.image) : null;
    if (pic) { row.append(pic); pic.firstElementChild!.addEventListener("load", () => { if (stick) el.scrollTop = el.scrollHeight; }); }
    el.append(row);
    while (el.childElementCount > MAX_LINES) el.firstElementChild!.remove();
    if (stick) el.scrollTop = el.scrollHeight; else showNew(true);
    const n = doc!.getElementById("chatCount");
    if (n) n.textContent = String(el.childElementCount);
    return row;
  }
  function system(text: string) { return add({ kind: "system", who: "", text }); }
  function error(text: string) { return add({ kind: "system", who: "", text, error: true }); }

  // The delivery status of the viewer's own talk line: 送信中… / 届いた / 送れなかった
  // (red, with the reason and a 再送 button).
  function setStatus(row: HTMLElement | undefined, status: string, label: string, why: string, retry?: () => void) {
    if (!row) return;
    let st = row.querySelector<HTMLSpanElement>(".st");
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
  function typing(name: string) {
    const t = doc && doc.getElementById("typing");
    if (!t) return;
    t.hidden = !name;
    t.textContent = name ? name + "が入力中…" : "";
    if (name && nearBottom()) logBox().scrollTop = logBox().scrollHeight;
  }

  // Open / close (the toggle only shows on narrow screens); it names what it does.
  const toggleLabel = (closed: boolean) => (closed ? "▲ ログを開く" : "▼ ログを閉じる");
  const toggle = doc && doc.getElementById("chatToggle");
  if (toggle) toggle.onclick = () => {
    const box = doc!.getElementById("chat")!;
    const closed = box.classList.toggle("closed");
    toggle.setAttribute("aria-expanded", String(!closed));
    toggle.textContent = toggleLabel(closed);
    if (!closed) logBox().scrollTop = logBox().scrollHeight;
    if (typeof CrabView !== "undefined") CrabView.layout();
  };

  // Phones: the page column (body.narrow) follows the visual viewport, so the
  // soft keyboard shrinks it from below and the chat panel -- log and input --
  // stays in view right above the keyboard. While typing with the keyboard up
  // the header rows fold away (body.kbup) so the log keeps its height.
  const vv = typeof window !== "undefined" && window.visualViewport;
  const box = doc && doc.getElementById("chat");
  const input = doc && doc.getElementById("talkText");
  const coarse = typeof window !== "undefined" && !!window.matchMedia && window.matchMedia("(pointer: coarse)").matches;
  function lift() {
    const b = doc!.body, narrow = b.classList.contains("narrow");
    const v = appViewport(window.innerHeight, vv);
    b.style.setProperty("--app-h", v.height + "px");
    b.style.setProperty("--vv-top", v.top + "px");
    if (vv) b.style.setProperty("--vv-h", Math.round(vv.height) + "px");
    const focused = !!input && doc!.activeElement === input;
    const on = narrow && focused && (coarse || keyboardInset(window.innerHeight, vv) > 0);
    if (b.classList.contains("kbup") !== on) {
      b.classList.toggle("kbup", on);
      if (on) logBox().scrollTop = logBox().scrollHeight;
    }
    if (narrow && window.scrollY) window.scrollTo(0, 0); // iOS scrolls the page to show the input: the column is already there
  }
  if (input && box) {
    input.addEventListener("focus", () => { box.classList.add("focus"); box.classList.remove("closed"); if (toggle) { toggle.textContent = toggleLabel(false); toggle.setAttribute("aria-expanded", "true"); } logBox().scrollTop = logBox().scrollHeight; lift(); setTimeout(lift, 300); });
    input.addEventListener("blur", () => { setTimeout(() => { box.classList.remove("focus"); lift(); }, 150); });
  }
  if (doc) { lift(); window.addEventListener("resize", lift); }
  if (vv && box) { vv.addEventListener("resize", lift); vv.addEventListener("scroll", lift); }

  // History across tabs: state events are ephemeral (kind 23411), relays keep
  // them for minutes at most, so the browser keeps the last talk/say/knock lines.
  const HIST_KEY = "crab-town-chat-v1", HIST_MAX = 200;
  const HIST_TYPES: Record<string, number> = { talk: 1, say: 1, knock: 1 };
  // one remembered line: the event id and the world message it carried
  interface HistItem { id: string; msg: WorldMsg }
  function keep(list: HistItem[], item: HistItem | null | undefined, max = HIST_MAX): HistItem[] {
    if (!item || !item.id || !item.msg || !HIST_TYPES[item.msg.type] || list.some(x => x.id === item.id)) return list;
    const out = list.concat([item]);
    return out.length > max ? out.slice(out.length - max) : out;
  }
  function loadHistory(store: Pick<Storage, "getItem"> | null | undefined): HistItem[] {
    try { const v: unknown = JSON.parse((store && store.getItem(HIST_KEY)) || "[]"); return Array.isArray(v) ? v.filter(x => x && x.id && x.msg && HIST_TYPES[x.msg.type]) : []; }
    catch { return []; }
  }
  function saveHistory(store: Pick<Storage, "setItem"> | null | undefined, list: HistItem[]) {
 try { store && store.setItem(HIST_KEY, JSON.stringify(list)); } catch { /* full / private mode */ } }

  return { entry, resultText, add, system, error, setStatus, typing, keyboardInset, toggleLabel, keep, loadHistory, saveHistory, appViewport, HIST_KEY };
})();
if (typeof module !== "undefined") module.exports = CrabChat;
