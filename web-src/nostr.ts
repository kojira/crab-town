// crab-town on GitHub Pages: talk to a crab-town running somewhere else through
// Nostr relays. Commands are ephemeral kind 23410 events signed with NIP-07
// (window.nostr); the town answers with kind 23411 events signed by its own key.
// Rendering is the same as the local viewer (sprites/furniture/floor/props/town/render.js).
// nostr-tools comes from web/nostr-tools.js (bundled locally, see web-src/vendor).
const { verifyEvent, decode, generateSecretKey, finalizeEvent } = NostrTools;
export {}; // a module (type="module" in nostr.html): its names stay out of the classic scripts' global scope

const KIND_COMMAND = 23410, KIND_STATE = 23411;
const cfg = window.CRAB_NOSTR || {};
const qs = new URLSearchParams(location.search);
const hexOf = (s: string | null | undefined): string => (s && s.startsWith("npub1") ? npubHex(s) : s || "").toLowerCase();
const TOWN = hexOf(qs.get("town") || cfg.town);
const RELAYS = (qs.get("relays") || (cfg.relays || []).join(",")).split(",").map(s => s.trim()).filter(Boolean);
// nostarou's room: the bed side of the bedroom (owner-only zone, so the public
// view frosts it: he walks in and disappears behind the curtain).
const ROOM_SPOT = cfg.roomSpot || { x: 29, y: 15 };

// npub1... -> hex (nip19 decode; anything but an npub is a config error)
function npubHex(s: string): string {
  const r = decode(s);
  if (r.type !== "npub") throw new Error("town: not an npub: " + s);
  return r.data;
}

const seen = new Set<string>(); // state event ids already applied (several relays deliver the same event)
// chat history kept in the browser (relays drop the ephemeral state events within minutes)
const store = (() => { try { return window.localStorage; } catch { return null; } })();
let chatHistory = CrabChat.loadHistory(store);
// one relay connection; eose = its stored events have all arrived
interface Sock { url: string; ws: WebSocket | null; open: boolean; eose?: boolean }
const sockets: Sock[] = [];
let me: string | null = null; // NIP-07 pubkey, once known
let myActor: string | null = null; // the owner's own avatar (set once the town answers "owner")
// "you" in the chat is only the logged-in pubkey's own talk. nostarou moves
// and speaks by itself (extgate); the owner walks their own avatar.
const LISTENING = cfg.listening || []; // actors someone receives knocks for (extgate)
const knockList = () => CrabViewport.knockTargets(Object.values(rooms)[0], actors, LISTENING);
const chatCtx = () => ({ selfGuestId: me ? "nostr:" + me.slice(0, 16) : null, houses: knockList() });

// the page's own elements (nostr.html): always there
const $ = <E extends HTMLElement = HTMLElement>(id: string) => document.getElementById(id) as E;
const relaysEl = $("relays");
function say(s: string) { CrabChat.system(s); }
function updateStatus() {
  statusEl.textContent = Object.values(actors)
    .map(a => a.hidden ? `${a.name}: (見えない場所にいる)` : `${a.name}: ${a.state} @ ${a.pos.x},${a.pos.y}`).join(" / ") || "waiting for the town...";
}
function inHidden(r: Room | undefined, p: Pos) {
  return !!r && (r.hidden_zones || []).some(id => {
    const z = (r.zones || []).find(z => z.id === id);
    return z && p && p.x >= z.rect.x && p.y >= z.rect.y && p.x < z.rect.x + z.rect.w && p.y < z.rect.y + z.rect.h;
  });
}

// apply one world message (same shapes as the local /world WebSocket)
function apply(ev: WorldMsg) {
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
    const mine = me && ev.by === ownAvatarId(me) && live && outbox.echo(ev.message || "", ev.image);
    if (mine) delivered(mine); else CrabChat.add(CrabChat.entry(ev, chatCtx()));
  } else if (ev.type === "say" && ev.actor) {
    CrabChat.add(CrabChat.entry(ev, chatCtx()));
    if (live && ev.actor.id === talkActor()) CrabChat.typing("");
    if (live && !ev.replay && typeof speech !== "undefined") speech[ev.actor.id] = { text: ev.message || "", until: performance.now() + CrabTalk.speechMs(ev.message) };
  } else if (ev.type === "result") {
    if (!me || ev.p !== me) { /* someone else's command */ }
    else if (ev.cmd === "talk" && outbox.result(ev.e, ev.ok, ev.error)) onTalkResult(ev);
    else if (live && CrabTalk.resultWorthALine(ev)) CrabChat.error(CrabChat.resultText(ev));
    if (me && ev.p === me && ev.role === "owner" && !myActor) { myActor = ownAvatarId(me); CrabView.setSelf(myActor); showImageButton(); }
  }
  updateStatus();
  updateKnockTargets();
  CrabView.onWorld();
}

// A logged-in person's actor carries its pubkey: fetch its kind:0 (r/n/x relays).
function loadAvatar(a: Actor | undefined) { if (a && a.pubkey) CrabAvatar.load(a.pubkey, RELAYS, verifyEvent); }
// the owner's avatar id: the same id its talk and knock carry
const ownAvatarId = (pk: string) => "nostr:" + pk.slice(0, 16);

// The knock menu lists every house; a house nobody listens at says so.
let knockKey = "";
function updateKnockTargets() {
  const ts = knockList(), key = ts.map(t => t.id + t.label + t.reachable).join("|");
  if (!ts.length || key === knockKey) return;
  knockKey = key;
  const sel = $<HTMLSelectElement>("knockTo"), cur = sel.value;
  sel.replaceChildren(...ts.map(t => new Option(t.label + (t.reachable ? "" : "（不在・記録のみ）"), t.id)));
  if (ts.some(t => t.id === cur)) sel.value = cur;
}

function onState(ev: NostrEvent) {
  if (ev.kind !== KIND_STATE || ev.pubkey !== TOWN || seen.has(ev.id)) return;
  if (!verifyEvent(ev)) { say("dropped a state event with a bad signature"); return; }
  seen.add(ev.id);
  if (seen.size > 5000) seen.clear();
  let msg: WorldMsg;
  try { msg = JSON.parse(ev.content); } catch { return; }
  const tag = (k: string): string | undefined => (ev.tags.find(t => t[0] === k) || [])[1];
  if (msg.type === "result") { msg.p = tag("p"); msg.e = tag("e"); }
  if (msg.type === "say" && !msg.reply_to && tag("reply_to")) msg.reply_to = tag("reply_to");
  const stamped = Object.assign(msg, { at: ev.created_at, created_at: ev.created_at });
  const kept = CrabChat.keep(chatHistory, { id: ev.id, msg });
  if (kept !== chatHistory) { chatHistory = kept; CrabChat.saveHistory(store, chatHistory); }
  // stored events replayed before EOSE arrive in any order: sort them first
  if (!live) { backlog.push(stamped); return; }

  applyOne(msg);
}
let snapAt = 0; // created_at of the newest snapshot applied
function applyOne(msg: WorldMsg) {
  if (msg.type === "snapshot" && (msg.created_at || 0) < snapAt) return; // older snapshot
  if (msg.type === "snapshot") snapAt = msg.created_at || 0;
  apply(msg);
}

// History: buffer until every open relay sent EOSE (or 2.5 s after the
// first one), then draw it oldest first. Later events are drawn as they come.
let live = false;
const backlog: (WorldMsg & { created_at: number })[] = [];
// lines saved by an earlier tab join the backlog (sorted with what the relays still have)
for (const h of chatHistory) { seen.add(h.id); backlog.push({ ...h.msg, created_at: h.msg.created_at || 0, replay: true }); }
let flushTimer: ReturnType<typeof setTimeout> | undefined;
function onEose(s: Sock) {
  s.eose = true;
  if (live) return;
  if (sockets.every(x => x.eose || !x.open)) flushBacklog();
  else if (!flushTimer) flushTimer = setTimeout(flushBacklog, 2500);
}
let loginTried = false;
function flushBacklog(force?: boolean) {
  if (live) return;
  // wait for the NIP-07 pubkey so history can say which lines were "you"
  if (!force && window.nostr && !loginTried) { setTimeout(flushBacklog, 300); return; }
  clearTimeout(flushTimer);
  live = true;
  for (const m of CrabTalk.sortBacklog(backlog.splice(0))) applyOne(m);
}
setTimeout(() => flushBacklog(true), 8000); // a relay that never sends EOSE does not hold the page

function connect(url: string) {
  const s: Sock = { url, ws: null, open: false };
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
    ws.onmessage = (m: MessageEvent<string>) => {
      let d: [string, ...unknown[]]; try { d = JSON.parse(m.data); } catch { return; }
      if (d[0] === "EVENT" && d[2]) onState(d[2] as NostrEvent);
      else if (d[0] === "EOSE") onEose(s);
      else if (d[0] === "OK" && d[2] === false) relayRejected(String(d[1]), url, String(d[3]));
    };
    ws.onclose = () => { s.open = false; s.eose = false; relaysEl.textContent = sockets.map(x => (x.open ? "●" : "○") + " " + x.url).join("  "); setTimeout(open, 3000); };
  };
  open();
}

// Commands need a signature. Without NIP-07 a throwaway key signs snapshot
// requests only (the town answers anyone with the public view).
let anonKey: Uint8Array | null = null;
type Template = { kind: number; created_at: number; tags: string[][]; content: string };
async function sign(tmpl: Template): Promise<NostrEvent> {
  if (window.nostr) return window.nostr.signEvent(tmpl);
  if (tmpl.content.includes('"snapshot"')) {
    anonKey ||= generateSecretKey();

    return finalizeEvent(tmpl, anonKey);
  }
  throw new Error("NIP-07 (window.nostr) が必要");
}

// Publish one command. Returns { ev, n } (n = relays it went to) or throws
// when it could not be signed. No line is written: callers report failures.
// a command to the town (kind 23410 content)
type Command = { type: string; [k: string]: unknown };
async function publish(cmd: Command) {
  const tmpl = { kind: KIND_COMMAND, created_at: Math.floor(Date.now() / 1000), tags: [["p", TOWN], ["t", "crab-town"]], content: JSON.stringify(cmd) };
  const ev = await sign(tmpl);
  let n = 0;
  for (const s of sockets) if (s.open && s.ws) { try { s.ws.send(JSON.stringify(["EVENT", ev])); n++; } catch { /* closing */ } }
  return { ev, n };
}
// Commands other than talk: only failures are worth a line.
const errText = (e: unknown) => (e instanceof Error && e.message) || String(e);
async function send(cmd: Command) {
  let r;
  try { r = await publish(cmd); } catch (e) { if (cmd.type !== "snapshot") CrabChat.error(`${cmd.type}: ${errText(e)}`); return; }
  if (r.n === 0 && cmd.type !== "snapshot") CrabChat.error(`${cmd.type}: つながっているリレーが無いので送れなかった`);
}

// ---- talk: shown at once, 送信中… -> 届いた, input cleared only when it is in ----
const outbox = CrabTalk.outbox();
const rows = new Map<number, HTMLElement | undefined>(); // outbox key -> log row
const TALK_TIMEOUT_MS = 20000, TYPING_MAX_MS = 120000;
const talkActor = () => cfg.talkTo || "nostarou";
const talkName = () => (actors[talkActor()] && actors[talkActor()].name) || "のすたろう";
type OutItem = ReturnType<typeof outbox.add>;
let typingTimer: ReturnType<typeof setTimeout> | undefined;
function showStatus(it: OutItem) {
  CrabChat.setStatus(rows.get(it.key), it.status, CrabTalk.STATUS_TEXT[it.status], it.error, () => talk(it.text, it, it.image));
}
const talkIn = $<HTMLInputElement>("talkText");
function delivered(it: OutItem) {
  showStatus(it);
  if (talkIn.value.trim() === it.text) talkIn.value = "";
  CrabChat.typing(talkName());
  clearTimeout(typingTimer);
  typingTimer = setTimeout(() => CrabChat.typing(""), TYPING_MAX_MS);
}
function failed(it: OutItem) { showStatus(it); talkIn.focus({ preventScroll: true }); }
function onTalkResult(ev: ResultMsg) {
  const it = outbox.items.find(x => x.eventId === ev.e);
  if (!it) return;
  if (it.status === "delivered") delivered(it); else failed(it);
}
function relayRejected(id: string, url: string, why: string) {
  const it = outbox.items.find(x => x.eventId === id);
  if (!it) { CrabChat.error(`${url} に拒否された: ${why}`); return; }
  // other relays may still carry it: the town's result decides
}
async function talk(text: string, again?: OutItem, image?: string) {
  const it = again || outbox.add(text, image);
  if (again) { it.status = "sending"; it.error = ""; it.eventId = ""; showStatus(it); }
  else rows.set(it.key, CrabChat.add({ kind: "self", who: "あなた", to: "", text, image }));
  showStatus(it);
  let r;
  try { r = await publish(image ? { type: "talk", text, image } : { type: "talk", text }); } catch (e) { outbox.signFailed(it, errText(e)); failed(it); return; }
  outbox.sent(it, r.ev.id, r.n);
  if (it.status === "failed") { failed(it); return; }
  const id = r.ev.id;
  setTimeout(() => {
    if (it.eventId === id && it.status === "sending") { it.status = "failed"; it.error = "町から応答が無い（届いていないかも）"; failed(it); }
  }, TALK_TIMEOUT_MS);
}

let snapTimer: ReturnType<typeof setTimeout> | undefined;
function requestSnapshot() {
  clearTimeout(snapTimer);
  snapTimer = setTimeout(() => send({ type: "snapshot" }), 300); // once for all relays opening together
}

// Without NIP-07 the input stays off; say why where the reader looks.
function setHint(loggedIn: boolean) {
  talkIn.placeholder = CrabTalk.inputHint(loggedIn);
  const h = document.getElementById("talkHint");
  if (h) h.hidden = loggedIn;
}
async function login() {
  if (!window.nostr) { CrabChat.error("NIP-07 拡張が見つからない。" + CrabTalk.HINT_LOGIN + "（今は閲覧のみ）"); setHint(false); return; }
  try { me = await window.nostr.getPublicKey(); } catch (e) { CrabChat.error("NIP-07: " + e); return; } finally { loginTried = true; }
  $("who").textContent = me.slice(0, 12) + "…";
  for (const b of document.querySelectorAll<HTMLButtonElement>(".cmd")) b.disabled = false;
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
  const to = $<HTMLSelectElement>("knockTo").value;
  const t = knockList().find(x => x.id === to);
  send({ type: "knock", room: to || "nostarou-house", message: "こんにちは" });
  if (t && !t.reachable) say(`${t.ownerName}は今つながっていないので、${t.label}へのノックは${t.ownerName}には届かない（町の出来事として記録されるだけ）`);
};
$("resync").onclick = () => send({ type: "snapshot" });
// talk: plain text, public, up to 280 characters (the town refuses longer)
$("talkForm").onsubmit = (e) => {
  e.preventDefault();
  const text = talkIn.value.trim();

  if (!text) return;
  if ([...text].length > 280) { CrabChat.error("talk: 280文字まで"); return; }
  talkIn.value = ""; // clear on send: the line is in the log (retry from there); waiting for the town's result left it in the box
  talk(text);
};

// ---- images (owner only, issue #20) ----
// The button appears only once the town has answered this pubkey as "owner";
// the town refuses an image from anyone else anyway. The file goes to Blossom
// (signed by the owner with NIP-07), the returned https URL rides in a talk.
const BLOSSOM = qs.get("blossom") || cfg.blossom || "https://blossom.primal.net";
const imgPick = $<HTMLInputElement>("imgPick"), imgBtn = $("imgBtn");
function showImageButton() { imgBtn.hidden = false; imgPick.disabled = false; }
imgPick.onchange = async () => {
  const file = imgPick.files && imgPick.files[0];
  imgPick.value = ""; // the same file may be picked again
  if (!file || !myActor || !window.nostr) return;
  const bad = CrabUpload.checkFile(file);
  if (bad) { CrabChat.error("画像: " + bad); return; }
  const signer = window.nostr;
  imgBtn.classList.add("busy"); imgBtn.setAttribute("aria-busy", "true");
  const note = CrabChat.system(`画像をアップロード中… (${Math.ceil(file.size / 1024)}KB → ${new URL(BLOSSOM).host})`);
  try {
    const url = await CrabUpload.upload(BLOSSOM, file, { fetch: (u: string, o: RequestInit) => fetch(u, o), sign: t => signer.signEvent(t), digest: d => crypto.subtle.digest("SHA-256", d) });
    note?.remove();
    const text = talkIn.value.trim();
    if ([...text].length > 280) { CrabChat.error("talk: 280文字まで"); return; }
    talk(text, undefined, url);
  } catch (e) { note?.remove(); CrabChat.error("画像: " + errText(e)); }
  finally { imgBtn.classList.remove("busy"); imgBtn.removeAttribute("aria-busy"); }
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
