// generated from web-src/nostr.ts by scripts/build-web.mjs; edit the .ts file, not this one
const { verifyEvent, decode, generateSecretKey, finalizeEvent } = NostrTools;
const KIND_COMMAND = 23410, KIND_STATE = 23411;
const cfg = window.CRAB_NOSTR || {};
const qs = new URLSearchParams(location.search);
const hexOf = (s) => (s && s.startsWith("npub1") ? npubHex(s) : s || "").toLowerCase();
const TOWN = hexOf(qs.get("town") || cfg.town);
const RELAYS = (qs.get("relays") || (cfg.relays || []).join(",")).split(",").map((s) => s.trim()).filter(Boolean);
const ROOM_SPOT = cfg.roomSpot || { x: 29, y: 15 };
function npubHex(s) {
  const r = decode(s);
  if (r.type !== "npub") throw new Error("town: not an npub: " + s);
  return r.data;
}
const seen = /* @__PURE__ */ new Set();
const store = (() => {
  try {
    return window.localStorage;
  } catch {
    return null;
  }
})();
let chatHistory = CrabChat.loadHistory(store);
const sockets = [];
let me = null;
let myActor = null;
const LISTENING = cfg.listening || [];
const knockList = () => CrabViewport.knockTargets(Object.values(rooms)[0], actors, LISTENING);
const chatCtx = () => ({ selfGuestId: me ? "nostr:" + me.slice(0, 16) : null, houses: knockList() });
const $ = (id) => document.getElementById(id);
const relaysEl = $("relays");
function say(s) {
  CrabChat.system(s);
}
function updateStatus() {
  statusEl.textContent = Object.values(actors).map((a) => a.hidden ? `${a.name}: (\u898B\u3048\u306A\u3044\u5834\u6240\u306B\u3044\u308B)` : `${a.name}: ${a.state} @ ${a.pos.x},${a.pos.y}`).join(" / ") || "waiting for the town...";
}
function inHidden(r, p) {
  return !!r && (r.hidden_zones || []).some((id) => {
    const z = (r.zones || []).find((z2) => z2.id === id);
    return z && p && p.x >= z.rect.x && p.y >= z.rect.y && p.x < z.rect.x + z.rect.w && p.y < z.rect.y + z.rect.h;
  });
}
function apply(ev) {
  if (ev.type === "snapshot") {
    rooms = {};
    actors = {};
    for (const r of ev.rooms) rooms[r.id] = r;
    const r0 = ev.rooms[0];
    if (r0 && (cv.width !== r0.width * T || cv.height !== r0.height * T)) {
      cv.width = r0.width * T;
      cv.height = r0.height * T;
    }
    for (const a of ev.actors) {
      actors[a.id] = a;
      loadAvatar(a);
    }
  } else if (ev.type === "actor" && ev.actor) {
    actors[ev.actor.id] = ev.actor;
    loadAvatar(ev.actor);
  } else if (ev.type === "occupancy") {
    const r = rooms[ev.room];
    if (r) {
      r.in_use = ev.in_use || [];
      r.hidden_zones = ev.hidden_zones || [];
    }
    for (const a of Object.values(actors)) if (a.room === ev.room && !a.hidden && inHidden(r, a.pos)) a.hidden = true;
  } else if (ev.type === "knock") {
    CrabChat.add(CrabChat.entry(ev, chatCtx()));
  } else if (ev.type === "talk") {
    const mine = me && ev.by === ownAvatarId(me) && live && outbox.echo(ev.message || "", ev.image);
    if (mine) delivered(mine);
    else CrabChat.add(CrabChat.entry(ev, chatCtx()));
  } else if (ev.type === "say" && ev.actor) {
    CrabChat.add(CrabChat.entry(ev, chatCtx()));
    if (live && ev.actor.id === talkActor()) CrabChat.typing("");
    if (live && !ev.replay && typeof speech !== "undefined") speech[ev.actor.id] = { text: ev.message || "", until: performance.now() + CrabTalk.speechMs(ev.message) };
  } else if (ev.type === "result") {
    if (!me || ev.p !== me) {
    } else if (ev.cmd === "talk" && outbox.result(ev.e, ev.ok, ev.error)) onTalkResult(ev);
    else if (live && CrabTalk.resultWorthALine(ev)) CrabChat.error(CrabChat.resultText(ev));
    if (me && ev.p === me && ev.role === "owner" && !myActor) {
      myActor = ownAvatarId(me);
      CrabView.setSelf(myActor);
      showImageButton();
    }
  }
  updateStatus();
  updateKnockTargets();
  CrabView.onWorld();
}
function loadAvatar(a) {
  if (a && a.pubkey) CrabAvatar.load(a.pubkey, RELAYS, verifyEvent);
}
const ownAvatarId = (pk) => "nostr:" + pk.slice(0, 16);
let knockKey = "";
function updateKnockTargets() {
  const ts = knockList(), key = ts.map((t) => t.id + t.label + t.reachable).join("|");
  if (!ts.length || key === knockKey) return;
  knockKey = key;
  const sel = $("knockTo"), cur = sel.value;
  sel.replaceChildren(...ts.map((t) => new Option(t.label + (t.reachable ? "" : "\uFF08\u4E0D\u5728\u30FB\u8A18\u9332\u306E\u307F\uFF09"), t.id)));
  if (ts.some((t) => t.id === cur)) sel.value = cur;
}
function onState(ev) {
  if (ev.kind !== KIND_STATE || ev.pubkey !== TOWN || seen.has(ev.id)) return;
  if (!verifyEvent(ev)) {
    say("dropped a state event with a bad signature");
    return;
  }
  seen.add(ev.id);
  if (seen.size > 5e3) seen.clear();
  let msg;
  try {
    msg = JSON.parse(ev.content);
  } catch {
    return;
  }
  const tag = (k) => (ev.tags.find((t) => t[0] === k) || [])[1];
  if (msg.type === "result") {
    msg.p = tag("p");
    msg.e = tag("e");
  }
  if (msg.type === "say" && !msg.reply_to && tag("reply_to")) msg.reply_to = tag("reply_to");
  const stamped = Object.assign(msg, { at: ev.created_at, created_at: ev.created_at });
  const kept = CrabChat.keep(chatHistory, { id: ev.id, msg });
  if (kept !== chatHistory) {
    chatHistory = kept;
    CrabChat.saveHistory(store, chatHistory);
  }
  if (!live) {
    backlog.push(stamped);
    return;
  }
  applyOne(msg);
}
let snapAt = 0;
function applyOne(msg) {
  if (msg.type === "snapshot" && (msg.created_at || 0) < snapAt) return;
  if (msg.type === "snapshot") snapAt = msg.created_at || 0;
  apply(msg);
}
let live = false;
const backlog = [];
for (const h of chatHistory) {
  seen.add(h.id);
  backlog.push({ ...h.msg, created_at: h.msg.created_at || 0, replay: true });
}
let flushTimer;
function onEose(s) {
  s.eose = true;
  if (live) return;
  if (sockets.every((x) => x.eose || !x.open)) flushBacklog();
  else if (!flushTimer) flushTimer = setTimeout(flushBacklog, 2500);
}
let loginTried = false;
function flushBacklog(force) {
  if (live) return;
  if (!force && window.nostr && !loginTried) {
    setTimeout(flushBacklog, 300);
    return;
  }
  clearTimeout(flushTimer);
  live = true;
  for (const m of CrabTalk.sortBacklog(backlog.splice(0))) applyOne(m);
}
setTimeout(() => flushBacklog(true), 8e3);
function connect(url) {
  const s = { url, ws: null, open: false };
  sockets.push(s);
  const open = () => {
    const ws = new WebSocket(url);
    s.ws = ws;
    ws.onopen = () => {
      s.open = true;
      ws.send(JSON.stringify(["REQ", "crab-state", { kinds: [KIND_STATE], authors: [TOWN], since: Math.floor(Date.now() / 1e3) - 60 }]));
      relaysEl.textContent = sockets.map((x) => (x.open ? "\u25CF" : "\u25CB") + " " + x.url).join("  ");
      requestSnapshot();
    };
    ws.onmessage = (m) => {
      let d;
      try {
        d = JSON.parse(m.data);
      } catch {
        return;
      }
      if (d[0] === "EVENT" && d[2]) onState(d[2]);
      else if (d[0] === "EOSE") onEose(s);
      else if (d[0] === "OK" && d[2] === false) relayRejected(String(d[1]), url, String(d[3]));
    };
    ws.onclose = () => {
      s.open = false;
      s.eose = false;
      relaysEl.textContent = sockets.map((x) => (x.open ? "\u25CF" : "\u25CB") + " " + x.url).join("  ");
      setTimeout(open, 3e3);
    };
  };
  open();
}
let anonKey = null;
async function sign(tmpl) {
  if (window.nostr) return window.nostr.signEvent(tmpl);
  if (tmpl.content.includes('"snapshot"')) {
    anonKey ||= generateSecretKey();
    return finalizeEvent(tmpl, anonKey);
  }
  throw new Error("NIP-07 (window.nostr) \u304C\u5FC5\u8981");
}
async function publish(cmd) {
  const tmpl = { kind: KIND_COMMAND, created_at: Math.floor(Date.now() / 1e3), tags: [["p", TOWN], ["t", "crab-town"]], content: JSON.stringify(cmd) };
  const ev = await sign(tmpl);
  let n = 0;
  for (const s of sockets) if (s.open && s.ws) {
    try {
      s.ws.send(JSON.stringify(["EVENT", ev]));
      n++;
    } catch {
    }
  }
  return { ev, n };
}
const errText = (e) => e instanceof Error && e.message || String(e);
async function send(cmd) {
  let r;
  try {
    r = await publish(cmd);
  } catch (e) {
    if (cmd.type !== "snapshot") CrabChat.error(`${cmd.type}: ${errText(e)}`);
    return;
  }
  if (r.n === 0 && cmd.type !== "snapshot") CrabChat.error(`${cmd.type}: \u3064\u306A\u304C\u3063\u3066\u3044\u308B\u30EA\u30EC\u30FC\u304C\u7121\u3044\u306E\u3067\u9001\u308C\u306A\u304B\u3063\u305F`);
}
const outbox = CrabTalk.outbox();
const rows = /* @__PURE__ */ new Map();
const TALK_TIMEOUT_MS = 2e4, TYPING_MAX_MS = 12e4;
const talkActor = () => cfg.talkTo || "nostarou";
const talkName = () => actors[talkActor()] && actors[talkActor()].name || "\u306E\u3059\u305F\u308D\u3046";
let typingTimer;
function showStatus(it) {
  CrabChat.setStatus(rows.get(it.key), it.status, CrabTalk.STATUS_TEXT[it.status], it.error, () => talk(it.text, it, it.image));
}
const talkIn = $("talkText");
function delivered(it) {
  showStatus(it);
  if (talkIn.value.trim() === it.text) talkIn.value = "";
  CrabChat.typing(talkName());
  clearTimeout(typingTimer);
  typingTimer = setTimeout(() => CrabChat.typing(""), TYPING_MAX_MS);
}
function failed(it) {
  showStatus(it);
  talkIn.focus({ preventScroll: true });
}
function onTalkResult(ev) {
  const it = outbox.items.find((x) => x.eventId === ev.e);
  if (!it) return;
  if (it.status === "delivered") delivered(it);
  else failed(it);
}
function relayRejected(id, url, why) {
  const it = outbox.items.find((x) => x.eventId === id);
  if (!it) {
    CrabChat.error(`${url} \u306B\u62D2\u5426\u3055\u308C\u305F: ${why}`);
    return;
  }
}
async function talk(text, again, image) {
  const it = again || outbox.add(text, image);
  if (again) {
    it.status = "sending";
    it.error = "";
    it.eventId = "";
    showStatus(it);
  } else rows.set(it.key, CrabChat.add({ kind: "self", who: "\u3042\u306A\u305F", to: "", text, image }));
  showStatus(it);
  let r;
  try {
    r = await publish(image ? { type: "talk", text, image } : { type: "talk", text });
  } catch (e) {
    outbox.signFailed(it, errText(e));
    failed(it);
    return;
  }
  outbox.sent(it, r.ev.id, r.n);
  if (it.status === "failed") {
    failed(it);
    return;
  }
  const id = r.ev.id;
  setTimeout(() => {
    if (it.eventId === id && it.status === "sending") {
      it.status = "failed";
      it.error = "\u753A\u304B\u3089\u5FDC\u7B54\u304C\u7121\u3044\uFF08\u5C4A\u3044\u3066\u3044\u306A\u3044\u304B\u3082\uFF09";
      failed(it);
    }
  }, TALK_TIMEOUT_MS);
}
let snapTimer;
function requestSnapshot() {
  clearTimeout(snapTimer);
  snapTimer = setTimeout(() => send({ type: "snapshot" }), 300);
}
function setHint(loggedIn) {
  talkIn.placeholder = CrabTalk.inputHint(loggedIn);
  const h = document.getElementById("talkHint");
  if (h) h.hidden = loggedIn;
}
async function login() {
  if (!window.nostr) {
    CrabChat.error("NIP-07 \u62E1\u5F35\u304C\u898B\u3064\u304B\u3089\u306A\u3044\u3002" + CrabTalk.HINT_LOGIN + "\uFF08\u4ECA\u306F\u95B2\u89A7\u306E\u307F\uFF09");
    setHint(false);
    return;
  }
  try {
    me = await window.nostr.getPublicKey();
  } catch (e) {
    CrabChat.error("NIP-07: " + e);
    return;
  } finally {
    loginTried = true;
  }
  $("who").textContent = me.slice(0, 12) + "\u2026";
  for (const b of document.querySelectorAll(".cmd")) b.disabled = false;
  setHint(true);
  CrabAvatar.load(me, RELAYS, verifyEvent);
  requestSnapshot();
}
cv.addEventListener("click", (e) => {
  if (!me) return;
  const r = cv.getBoundingClientRect();
  const x = Math.floor((e.clientX - r.left) * cv.width / r.width / T);
  const y = Math.floor((e.clientY - r.top) * cv.height / r.height / T);
  send({ type: "move", x, y });
});
$("login").onclick = login;
$("toRoom").onclick = () => send({ type: "move", ...ROOM_SPOT });
$("toGarden").onclick = () => send({ type: "move", x: 24, y: 4 });
$("knock").onclick = () => {
  const to = $("knockTo").value;
  const t = knockList().find((x) => x.id === to);
  send({ type: "knock", room: to || "nostarou-house", message: "\u3053\u3093\u306B\u3061\u306F" });
  if (t && !t.reachable) say(`${t.ownerName}\u306F\u4ECA\u3064\u306A\u304C\u3063\u3066\u3044\u306A\u3044\u306E\u3067\u3001${t.label}\u3078\u306E\u30CE\u30C3\u30AF\u306F${t.ownerName}\u306B\u306F\u5C4A\u304B\u306A\u3044\uFF08\u753A\u306E\u51FA\u6765\u4E8B\u3068\u3057\u3066\u8A18\u9332\u3055\u308C\u308B\u3060\u3051\uFF09`);
};
$("resync").onclick = () => send({ type: "snapshot" });
$("talkForm").onsubmit = (e) => {
  e.preventDefault();
  const text = talkIn.value.trim();
  if (!text) return;
  if ([...text].length > 280) {
    CrabChat.error("talk: 280\u6587\u5B57\u307E\u3067");
    return;
  }
  talkIn.value = "";
  talk(text);
};
const BLOSSOM = qs.get("blossom") || cfg.blossom || "https://blossom.primal.net";
const imgPick = $("imgPick"), imgBtn = $("imgBtn");
function showImageButton() {
  imgBtn.hidden = false;
  imgPick.disabled = false;
}
imgPick.onchange = async () => {
  const file = imgPick.files && imgPick.files[0];
  imgPick.value = "";
  if (!file || !myActor || !window.nostr) return;
  const bad = CrabUpload.checkFile(file);
  if (bad) {
    CrabChat.error("\u753B\u50CF: " + bad);
    return;
  }
  const signer = window.nostr;
  imgBtn.classList.add("busy");
  imgBtn.setAttribute("aria-busy", "true");
  const note = CrabChat.system(`\u753B\u50CF\u3092\u30A2\u30C3\u30D7\u30ED\u30FC\u30C9\u4E2D\u2026 (${Math.ceil(file.size / 1024)}KB \u2192 ${new URL(BLOSSOM).host})`);
  try {
    const url = await CrabUpload.upload(BLOSSOM, file, { fetch: (u, o) => fetch(u, o), sign: (t) => signer.signEvent(t), digest: (d) => crypto.subtle.digest("SHA-256", d) });
    note?.remove();
    const text = talkIn.value.trim();
    if ([...text].length > 280) {
      CrabChat.error("talk: 280\u6587\u5B57\u307E\u3067");
      return;
    }
    talk(text, void 0, url);
  } catch (e) {
    note?.remove();
    CrabChat.error("\u753B\u50CF: " + errText(e));
  } finally {
    imgBtn.classList.remove("busy");
    imgBtn.removeAttribute("aria-busy");
  }
};
if (!TOWN || !/^[0-9a-f]{64}$/.test(TOWN) || RELAYS.length === 0) {
  statusEl.textContent = "town pubkey / relays not configured (config.js or ?town=&relays=)";
} else {
  $("town").textContent = TOWN.slice(0, 12) + "\u2026";
  RELAYS.forEach(connect);
  updateStatus();
}
setHint(false);
setTimeout(() => {
  if (window.nostr) login();
}, 500);
requestAnimationFrame(draw);
