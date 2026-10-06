// generated from web-src/chatlog.ts by scripts/build-web.mjs; edit the .ts file, not this one
const CrabChat = (() => {
  const MAX_LINES = 200;
  function entry(ev, ctx = {}) {
    if (!ev) return null;
    const at = ev.at || 0;
    if (ev.type === "say" && ev.actor) {
      const id = ev.actor.id;
      const you = !!ctx.selfGuestId && ev.reply_to === ctx.selfGuestId;
      const other = !!ev.reply_to && !you;
      return {
        kind: "resident",
        who: ev.actor.name || id,
        actor: id,
        text: ev.message || "",
        at,
        to: you ? "\u3042\u306A\u305F" : other && ctx.selfGuestId ? "\u6765\u5BA2 " + short(ev.reply_to) : "",
        dim: other && !!ctx.selfGuestId
      };
    }
    if (ev.type === "talk") {
      const self = !!ctx.selfGuestId && ev.by === ctx.selfGuestId;
      const kind = self ? "self" : ev.role === "owner" ? "owner" : "guest";
      const who = self ? "\u3042\u306A\u305F" : kind === "owner" ? "\u30AA\u30FC\u30CA\u30FC" : "\u6765\u5BA2 " + short(ev.by);
      const image = ev.role === "owner" && ev.image ? ev.image : void 0;
      return { kind, who, to: ev.to || "", text: ev.message || "", at, dim: !self && !!ctx.selfGuestId, image };
    }
    if (ev.type === "knock") {
      const h = (ctx.houses || []).find((x) => x.id === ev.house);
      const by = ctx.selfGuestId && ev.by === ctx.selfGuestId ? "\u3042\u306A\u305F" : String(ev.by || "").startsWith("nostr:") ? "\u6765\u5BA2 " + short(ev.by) : ev.by;
      let text = `${by} \u2192 ${h ? h.label : ev.house || ev.room} ${ev.message || ""}`.trim();
      if (h && !h.reachable) text += `\uFF08${h.ownerName}\u306F\u4ECA\u3064\u306A\u304C\u3063\u3066\u3044\u306A\u3044\u306E\u3067\u5C4A\u304B\u306A\u3044\u3002\u753A\u306E\u51FA\u6765\u4E8B\u3068\u3057\u3066\u8A18\u9332\u3060\u3051\uFF09`;
      return { kind: "system", who: "\u30CE\u30C3\u30AF", text, at };
    }
    return null;
  }
  function short(by) {
    return String(by || "").replace(/^nostr:/, "").slice(0, 8);
  }
  function resultText(ev) {
    if (ev.ok) return `${ev.cmd}: ok (${ev.role})`;
    const why = /forbidden/.test(ev.error || "") ? ev.cmd === "move" ? "\u305D\u306E\u5834\u6240\u306B\u306F\u5165\u308C\u306A\u3044\uFF08\u62DB\u5F85\u3055\u308C\u3066\u3044\u306A\u3044\u5BB6\u30FB\u898B\u3048\u306A\u3044\u90E8\u5C4B\uFF09" : "\u6A29\u9650\u304C\u306A\u3044" : ev.error;
    return `${ev.cmd}: NG ${why} (${ev.role})`;
  }
  function keyboardInset(innerH, vv2) {
    if (!vv2 || !(innerH > 0)) return 0;
    const d = innerH - (vv2.height + (vv2.offsetTop || 0));
    return d > 1 ? Math.round(d) : 0;
  }
  const doc = typeof document !== "undefined" ? document : null;
  const el = doc && doc.getElementById("log");
  function nearBottom() {
    const el2 = logBox();
    return el2.scrollHeight - el2.scrollTop - el2.clientHeight < 24;
  }
  const newBtn = doc && doc.getElementById("newLines");
  const showNew = (on) => {
    if (newBtn) newBtn.hidden = !on;
  };
  if (newBtn) newBtn.onclick = () => {
    const el2 = logBox();
    el2.scrollTop = el2.scrollHeight;
    showNew(false);
  };
  if (el) el.addEventListener("scroll", () => {
    if (nearBottom()) showNew(false);
  });
  const hhmm = (at) => (at ? new Date(at * 1e3) : /* @__PURE__ */ new Date()).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  function logBox() {
    return el;
  }
  function add(e) {
    if (!e || !el) return;
    const stick = nearBottom();
    const row = document.createElement("div");
    row.className = "line " + e.kind + (e.actor ? " a-" + e.actor.replace(/[^a-z0-9_-]/gi, "") : "") + (e.dim ? " dim" : "") + (e.error ? " err" : "");
    const t = document.createElement("span");
    t.className = "t";
    t.textContent = hhmm(e.at);
    const w = document.createElement("span");
    w.className = "who";
    w.textContent = e.who || "";
    const m = document.createElement("span");
    m.className = "msg";
    m.textContent = (e.to ? "\u2192 " + e.to + ": " : "") + (e.text || "");
    row.append(t, " ", w, " ", m);
    const pic = e.image && typeof CrabUpload !== "undefined" ? CrabUpload.imageNode(document, e.image) : null;
    if (pic) {
      row.append(pic);
      pic.firstElementChild.addEventListener("load", () => {
        if (stick) el.scrollTop = el.scrollHeight;
      });
    }
    el.append(row);
    while (el.childElementCount > MAX_LINES) el.firstElementChild.remove();
    if (stick) el.scrollTop = el.scrollHeight;
    else showNew(true);
    const n = doc.getElementById("chatCount");
    if (n) n.textContent = String(el.childElementCount);
    return row;
  }
  function system(text) {
    return add({ kind: "system", who: "", text });
  }
  function error(text) {
    return add({ kind: "system", who: "", text, error: true });
  }
  function setStatus(row, status, label, why, retry) {
    if (!row) return;
    let st = row.querySelector(".st");
    if (!st) {
      st = document.createElement("span");
      row.append(" ", st);
    }
    st.className = "st " + status;
    st.textContent = status === "failed" ? `${label}: ${why}` : label;
    if (status === "failed" && retry) {
      const b = document.createElement("button");
      b.type = "button";
      b.className = "retry";
      b.textContent = "\u518D\u9001";
      b.onclick = retry;
      st.append(" ", b);
    }
  }
  function typing(name) {
    const t = doc && doc.getElementById("typing");
    if (!t) return;
    t.hidden = !name;
    t.textContent = name ? name + "\u304C\u5165\u529B\u4E2D\u2026" : "";
    if (name && nearBottom()) logBox().scrollTop = logBox().scrollHeight;
  }
  const toggleLabel = (closed) => closed ? "\u25B2 \u30ED\u30B0\u3092\u958B\u304F" : "\u25BC \u30ED\u30B0\u3092\u9589\u3058\u308B";
  const toggle = doc && doc.getElementById("chatToggle");
  if (toggle) toggle.onclick = () => {
    const box2 = doc.getElementById("chat");
    const closed = box2.classList.toggle("closed");
    toggle.setAttribute("aria-expanded", String(!closed));
    toggle.textContent = toggleLabel(closed);
    if (!closed) logBox().scrollTop = logBox().scrollHeight;
    if (typeof CrabView !== "undefined") CrabView.layout();
  };
  const vv = typeof window !== "undefined" && window.visualViewport;
  const box = doc && doc.getElementById("chat");
  function lift() {
    const k = doc.body.classList.contains("narrow") ? keyboardInset(window.innerHeight, vv) : 0;
    box.style.bottom = k ? k + "px" : "";
    if (vv) doc.body.style.setProperty("--vv-h", Math.round(vv.height) + "px");
  }
  const input = doc && doc.getElementById("talkText");
  if (input && box) {
    input.addEventListener("focus", () => {
      box.classList.add("focus");
      box.classList.remove("closed");
      if (toggle) {
        toggle.textContent = toggleLabel(false);
        toggle.setAttribute("aria-expanded", "true");
      }
      logBox().scrollTop = logBox().scrollHeight;
      lift();
    });
    input.addEventListener("blur", () => {
      setTimeout(() => box.classList.remove("focus"), 150);
    });
  }
  if (vv && box) {
    vv.addEventListener("resize", lift);
    vv.addEventListener("scroll", lift);
  }
  const HIST_KEY = "crab-town-chat-v1", HIST_MAX = 200;
  const HIST_TYPES = { talk: 1, say: 1, knock: 1 };
  function keep(list, item, max = HIST_MAX) {
    if (!item || !item.id || !item.msg || !HIST_TYPES[item.msg.type] || list.some((x) => x.id === item.id)) return list;
    const out = list.concat([item]);
    return out.length > max ? out.slice(out.length - max) : out;
  }
  function loadHistory(store) {
    try {
      const v = JSON.parse(store && store.getItem(HIST_KEY) || "[]");
      return Array.isArray(v) ? v.filter((x) => x && x.id && x.msg && HIST_TYPES[x.msg.type]) : [];
    } catch {
      return [];
    }
  }
  function saveHistory(store, list) {
    try {
      store && store.setItem(HIST_KEY, JSON.stringify(list));
    } catch {
    }
  }
  return { entry, resultText, add, system, error, setStatus, typing, keyboardInset, toggleLabel, keep, loadHistory, saveHistory, HIST_KEY };
})();
if (typeof module !== "undefined") module.exports = CrabChat;
