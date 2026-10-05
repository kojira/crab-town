// crab-town on GitHub Pages: talk to a crab-town running somewhere else through
// Nostr relays. Commands are ephemeral kind 23410 events signed with NIP-07
// (window.nostr); the town answers with kind 23411 events signed by its own key.
// Rendering is the same as the local viewer (sprites/furniture/floor/props/town/render.js).
import { verifyEvent } from "https://esm.sh/nostr-tools@2.17.0/pure";
import { decode } from "https://esm.sh/nostr-tools@2.17.0/nip19";

const KIND_COMMAND = 23410, KIND_STATE = 23411;
const cfg = window.CRAB_NOSTR || {};
const qs = new URLSearchParams(location.search);
const hexOf = (s) => (s && s.startsWith("npub1") ? decode(s).data : s || "").toLowerCase();
const TOWN = hexOf(qs.get("town") || cfg.town);
const RELAYS = (qs.get("relays") || (cfg.relays || []).join(",")).split(",").map(s => s.trim()).filter(Boolean);
// nostarou's room: the bed side of the bedroom (owner-only zone, so the public
// view frosts it: he walks in and disappears behind the curtain).
const ROOM_SPOT = cfg.roomSpot || { x: 29, y: 15 };

const seen = new Set(); // state event ids already applied (several relays deliver the same event)
const sockets = [];
let me = null; // NIP-07 pubkey, once known
let myActor = null; // the owner's own avatar (set once the town answers "owner")
// "you" in the chat is only the logged-in pubkey's own talk. nostarou moves
// and speaks by itself (extgate); the owner walks their own avatar.
const LISTENING = cfg.listening || []; // actors someone receives knocks for (extgate)
const knockList = () => CrabViewport.knockTargets(Object.values(rooms)[0], actors, LISTENING);
const chatCtx = () => ({ selfGuestId: me ? "nostr:" + me.slice(0, 16) : null, houses: knockList() });

const $ = (id) => document.getElementById(id);
const relaysEl = $("relays");
function say(s) { CrabChat.system(s); }
function updateStatus() {
  statusEl.textContent = Object.values(actors)
    .map(a => a.hidden ? `${a.name}: (見えない場所にいる)` : `${a.name}: ${a.state} @ ${a.pos.x},${a.pos.y}`).join(" / ") || "waiting for the town...";
}
function inHidden(r, p) {
  return !!r && (r.hidden_zones || []).some(id => {
    const z = (r.zones || []).find(z => z.id === id);
    return z && p && p.x >= z.rect.x && p.y >= z.rect.y && p.x < z.rect.x + z.rect.w && p.y < z.rect.y + z.rect.h;
  });
}

// apply one world message (same shapes as the local /world WebSocket)
function apply(ev) {
  if (ev.type === "snapshot") {
    rooms = {}; actors = {};
    for (const r of ev.rooms) rooms[r.id] = r;
    const r0 = ev.rooms[0];
    if (r0 && (cv.width !== r0.width * T || cv.height !== r0.height * T)) { cv.width = r0.width * T; cv.height = r0.height * T; }
    for (const a of ev.actors) { actors[a.id] = a; loadAvatar(a); }
  } else if (ev.type === "actor" && ev.actor) {
    actors[ev.actor.id] = ev.actor;
    loadAvatar(ev.actor);
  } else if (ev.type === "occupancy") {
    const r = rooms[ev.room];
    if (r) { r.in_use = ev.in_use || []; r.hidden_zones = ev.hidden_zones || []; }
    for (const a of Object.values(actors)) if (a.room === ev.room && !a.hidden && inHidden(r, a.pos)) a.hidden = true;
  } else if (ev.type === "knock") {
    CrabChat.add(CrabChat.entry(ev, chatCtx()));
  } else if (ev.type === "talk") {
    // our own talk is already on screen (sent at once): the echo only confirms it
    const mine = me && ev.by === ownAvatarId(me) && live && outbox.echo(ev.message || "");
    if (mine) delivered(mine); else CrabChat.add(CrabChat.entry(ev, chatCtx()));
  } else if (ev.type === "say" && ev.actor) {
    CrabChat.add(CrabChat.entry(ev, chatCtx()));
    if (live && ev.actor.id === talkActor()) CrabChat.typing("");
    if (live && typeof speech !== "undefined") speech[ev.actor.id] = { text: ev.message || "", until: performance.now() + CrabTalk.speechMs(ev.message) };
  } else if (ev.type === "result") {
    if (!me || ev.p !== me) { /* someone else's command */ }
    else if (ev.cmd === "talk" && outbox.result(ev.e, ev.ok, ev.error)) onTalkResult(ev);
    else if (live && CrabTalk.resultWorthALine(ev)) CrabChat.error(CrabChat.resultText(ev));
    if (me && ev.p === me && ev.role === "owner" && !myActor) { myActor = ownAvatarId(me); CrabView.setSelf(myActor); }
  }
  updateStatus();
  updateKnockTargets();
  CrabView.onWorld();
}

// A logged-in person's actor carries its pubkey: fetch its kind:0 (r/n/x relays).
function loadAvatar(a) { if (a && a.pubkey) CrabAvatar.load(a.pubkey, RELAYS, verifyEvent); }
// the owner's avatar id: the same id its talk and knock carry
const ownAvatarId = (pk) => "nostr:" + pk.slice(0, 16);

// The knock menu lists every house; a house nobody listens at says so.
let knockKey = "";
function updateKnockTargets() {
  const ts = knockList(), key = ts.map(t => t.id + t.label + t.reachable).join("|");
  if (!ts.length || key === knockKey) return;
  knockKey = key;
  const sel = $("knockTo"), cur = sel.value;
  sel.replaceChildren(...ts.map(t => new Option(t.label + (t.reachable ? "" : "（不在・記録のみ）"), t.id)));
  if (ts.some(t => t.id === cur)) sel.value = cur;
}

function onState(ev) {
  if (ev.kind !== KIND_STATE || ev.pubkey !== TOWN || seen.has(ev.id)) return;
  if (!verifyEvent(ev)) { say("dropped a state event with a bad signature"); return; }
  seen.add(ev.id);
  if (seen.size > 5000) seen.clear();
  let msg;
  try { msg = JSON.parse(ev.content); } catch { return; }
  const tag = (k) => (ev.tags.find(t => t[0] === k) || [])[1];
  if (msg.type === "result") { msg.p = tag("p"); msg.e = tag("e"); }
  if (msg.type === "say" && !msg.reply_to && tag("reply_to")) msg.reply_to = tag("reply_to");
  msg.at = ev.created_at; msg.created_at = ev.created_at;
  // stored events replayed before EOSE arrive in any order: sort them first
  if (!live) { backlog.push(msg); return; }
  applyOne(msg);
}
function applyOne(msg) {
  if (msg.type === "snapshot" && msg.created_at < (applyOne.snapAt || 0)) return; // older snapshot
  if (msg.type === "snapshot") applyOne.snapAt = msg.created_at;
  apply(msg);
}

// History: buffer until every open relay sent EOSE (or 2.5 s after the
// first one), then draw it oldest first. Later events are drawn as they come.
let live = false;
const backlog = [];
let flushTimer = null;
function onEose(s) {
  s.eose = true;
  if (live) return;
  if (sockets.every(x => x.eose || !x.open)) flushBacklog();
  else if (!flushTimer) flushTimer = setTimeout(flushBacklog, 2500);
}
let loginTried = false;
function flushBacklog(force) {
  if (live) return;
  // wait for the NIP-07 pubkey so history can say which lines were "you"
  if (!force && window.nostr && !loginTried) { setTimeout(flushBacklog, 300); return; }
  clearTimeout(flushTimer);
  live = true;
  for (const m of CrabTalk.sortBacklog(backlog.splice(0))) applyOne(m);
}
setTimeout(() => flushBacklog(true), 8000); // a relay that never sends EOSE does not hold the page

function connect(url) {
  const s = { url, ws: null, open: false };
  sockets.push(s);
  const open = () => {
    const ws = new WebSocket(url);
    s.ws = ws;
    ws.onopen = () => {
      s.open = true;
      ws.send(JSON.stringify(["REQ", "crab-state", { kinds: [KIND_STATE], authors: [TOWN], since: Math.floor(Date.now() / 1000) - 60 }]));
      relaysEl.textContent = sockets.map(x => (x.open ? "●" : "○") + " " + x.url).join("  ");
      requestSnapshot();
    };
    ws.onmessage = (m) => {
      let d; try { d = JSON.parse(m.data); } catch { return; }
      if (d[0] === "EVENT" && d[2]) onState(d[2]);
      else if (d[0] === "EOSE") onEose(s);
      else if (d[0] === "OK" && d[2] === false) relayRejected(d[1], url, d[3]);
    };
    ws.onclose = () => { s.open = false; s.eose = false; relaysEl.textContent = sockets.map(x => (x.open ? "●" : "○") + " " + x.url).join("  "); setTimeout(open, 3000); };
  };
  open();
}

// Commands need a signature. Without NIP-07 a throwaway key signs snapshot
// requests only (the town answers anyone with the public view).
let anonKey = null;
async function sign(tmpl) {
  if (window.nostr) return window.nostr.signEvent(tmpl);
  if (tmpl.content.includes('"snapshot"')) {
    const { generateSecretKey, finalizeEvent } = await import("https://esm.sh/nostr-tools@2.17.0/pure");
    anonKey ||= generateSecretKey();
    return finalizeEvent(tmpl, anonKey);
  }
  throw new Error("NIP-07 (window.nostr) が必要");
}

// Publish one command. Returns { ev, n } (n = relays it went to) or throws
// when it could not be signed. No line is written: callers report failures.
async function publish(cmd) {
  const tmpl = { kind: KIND_COMMAND, created_at: Math.floor(Date.now() / 1000), tags: [["p", TOWN], ["t", "crab-town"]], content: JSON.stringify(cmd) };
  const ev = await sign(tmpl);
  let n = 0;
  for (const s of sockets) if (s.open) { try { s.ws.send(JSON.stringify(["EVENT", ev])); n++; } catch { /* closing */ } }
  return { ev, n };
}
// Commands other than talk: only failures are worth a line.
async function send(cmd) {
  let r;
  try { r = await publish(cmd); } catch (e) { if (cmd.type !== "snapshot") CrabChat.error(`${cmd.type}: ${e.message || e}`); return; }
  if (r.n === 0 && cmd.type !== "snapshot") CrabChat.error(`${cmd.type}: つながっているリレーが無いので送れなかった`);
}

// ---- talk: shown at once, 送信中… -> 届いた, input cleared only when it is in ----
const outbox = CrabTalk.outbox();
const rows = new Map(); // outbox key -> log row
const TALK_TIMEOUT_MS = 20000, TYPING_MAX_MS = 120000;
const talkActor = () => cfg.talkTo || "nostarou";
const talkName = () => (actors[talkActor()] && actors[talkActor()].name) || "のすたろう";
let typingTimer = null;
function showStatus(it) {
  CrabChat.setStatus(rows.get(it.key), it.status, CrabTalk.STATUS_TEXT[it.status], it.error, () => talk(it.text, it));
}
function delivered(it) {
  showStatus(it);
  if ($("talkText").value.trim() === it.text) $("talkText").value = "";
  CrabChat.typing(talkName());
  clearTimeout(typingTimer);
  typingTimer = setTimeout(() => CrabChat.typing(""), TYPING_MAX_MS);
}
function failed(it) { showStatus(it); $("talkText").focus({ preventScroll: true }); }
function onTalkResult(ev) {
  const it = outbox.items.find(x => x.eventId === ev.e);
  if (it.status === "delivered") delivered(it); else failed(it);
}
function relayRejected(id, url, why) {
  const it = outbox.items.find(x => x.eventId === id);
  if (!it) { CrabChat.error(`${url} に拒否された: ${why}`); return; }
  // other relays may still carry it: the town's result decides
}
async function talk(text, again) {
  const it = again || outbox.add(text);
  if (again) { it.status = "sending"; it.error = ""; it.eventId = ""; showStatus(it); }
  else rows.set(it.key, CrabChat.add({ kind: "self", who: "あなた", to: "", text }));
  showStatus(it);
  let r;
  try { r = await publish({ type: "talk", text }); } catch (e) { outbox.signFailed(it, e.message || e); failed(it); return; }
  outbox.sent(it, r.ev.id, r.n);
  if (it.status === "failed") { failed(it); return; }
  const id = r.ev.id;
  setTimeout(() => {
    if (it.eventId === id && it.status === "sending") { it.status = "failed"; it.error = "町から応答が無い（届いていないかも）"; failed(it); }
  }, TALK_TIMEOUT_MS);
}

let snapTimer = null;
function requestSnapshot() {
  clearTimeout(snapTimer);
  snapTimer = setTimeout(() => send({ type: "snapshot" }), 300); // once for all relays opening together
}

// Without NIP-07 the input stays off; say why where the reader looks.
function setHint(loggedIn) {
  $("talkText").placeholder = CrabTalk.inputHint(loggedIn);
  const h = $("talkHint");
  if (h) h.hidden = loggedIn;
}
async function login() {
  if (!window.nostr) { CrabChat.error("NIP-07 拡張が見つからない。" + CrabTalk.HINT_LOGIN + "（今は閲覧のみ）"); setHint(false); return; }
  try { me = await window.nostr.getPublicKey(); } catch (e) { CrabChat.error("NIP-07: " + e); return; } finally { loginTried = true; }
  $("who").textContent = me.slice(0, 12) + "…";
  for (const b of document.querySelectorAll(".cmd")) b.disabled = false;
  setHint(true);
  CrabAvatar.load(me, RELAYS, verifyEvent);
  requestSnapshot(); // a signed command: the owner's avatar joins the garden
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
  const t = knockList().find(x => x.id === $("knockTo").value);
  send({ type: "knock", room: $("knockTo").value || "nostarou-house", message: "こんにちは" });
  if (t && !t.reachable) say(`${t.ownerName}は今つながっていないので、${t.label}へのノックは${t.ownerName}には届かない（町の出来事として記録されるだけ）`);
};
$("resync").onclick = () => send({ type: "snapshot" });
// talk: plain text, public, up to 280 characters (the town refuses longer)
$("talkForm").onsubmit = (e) => {
  e.preventDefault();
  const text = $("talkText").value.trim();
  if (!text) return;
  if ([...text].length > 280) { CrabChat.error("talk: 280文字まで"); return; }
  talk(text);
};

if (!TOWN || !/^[0-9a-f]{64}$/.test(TOWN) || RELAYS.length === 0) {
  statusEl.textContent = "town pubkey / relays not configured (config.js or ?town=&relays=)";
} else {
  $("town").textContent = TOWN.slice(0, 12) + "…";
  RELAYS.forEach(connect);
  updateStatus();
}
setHint(false);
setTimeout(() => { if (window.nostr) login(); }, 500);
requestAnimationFrame(draw);
