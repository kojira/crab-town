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

const $ = (id) => document.getElementById(id);
const relaysEl = $("relays");
function say(s) { logEl.textContent = (new Date().toLocaleTimeString() + " " + s + "\n" + logEl.textContent).slice(0, 4000); }
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
    for (const a of ev.actors) actors[a.id] = a;
  } else if (ev.type === "actor" && ev.actor) {
    actors[ev.actor.id] = ev.actor;
  } else if (ev.type === "occupancy") {
    const r = rooms[ev.room];
    if (r) { r.in_use = ev.in_use || []; r.hidden_zones = ev.hidden_zones || []; }
    for (const a of Object.values(actors)) if (a.room === ev.room && !a.hidden && inHidden(r, a.pos)) a.hidden = true;
  } else if (ev.type === "knock") {
    say(`knock: ${ev.by} → ${ev.house || ev.room} ${ev.message || ""}`);
  } else if (ev.type === "talk") {
    say(`talk: ${ev.by}${ev.role ? " (" + ev.role + ")" : ""} → ${ev.to}: ${ev.message || ""}`);
  } else if (ev.type === "say" && ev.actor) {
    say(`say: ${ev.actor.name || ev.actor.id}: ${ev.message || ""}`);
    if (typeof speech !== "undefined") speech[ev.actor.id] = { text: ev.message || "", until: performance.now() + SPEECH_MS };
  } else if (ev.type === "result") {
    if (!me || ev.p === me) say(`${ev.cmd}: ${ev.ok ? "ok" : "NG " + ev.error} (${ev.role})`);
  }
  updateStatus();
}

function onState(ev) {
  if (ev.kind !== KIND_STATE || ev.pubkey !== TOWN || seen.has(ev.id)) return;
  if (!verifyEvent(ev)) { say("dropped a state event with a bad signature"); return; }
  seen.add(ev.id);
  if (seen.size > 5000) seen.clear();
  let msg;
  try { msg = JSON.parse(ev.content); } catch { return; }
  const p = (ev.tags.find(t => t[0] === "p") || [])[1];
  if (msg.type === "result") msg.p = p;
  if (msg.type === "snapshot" && ev.created_at < (onState.snapAt || 0)) return; // older snapshot
  if (msg.type === "snapshot") onState.snapAt = ev.created_at;
  apply(msg);
}

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
      else if (d[0] === "OK" && d[2] === false) say(`${url} rejected: ${d[3]}`);
    };
    ws.onclose = () => { s.open = false; relaysEl.textContent = sockets.map(x => (x.open ? "●" : "○") + " " + x.url).join("  "); setTimeout(open, 3000); };
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

async function send(cmd) {
  const tmpl = { kind: KIND_COMMAND, created_at: Math.floor(Date.now() / 1000), tags: [["p", TOWN], ["t", "crab-town"]], content: JSON.stringify(cmd) };
  let ev;
  try { ev = await sign(tmpl); } catch (e) { say(`${cmd.type}: ${e.message || e}`); return; }
  let n = 0;
  for (const s of sockets) if (s.open) { s.ws.send(JSON.stringify(["EVENT", ev])); n++; }
  if (cmd.type !== "snapshot") say(`sent ${cmd.type} to ${n} relay(s)`);
}

let snapTimer = null;
function requestSnapshot() {
  clearTimeout(snapTimer);
  snapTimer = setTimeout(() => send({ type: "snapshot" }), 300); // once for all relays opening together
}

async function login() {
  if (!window.nostr) { say("NIP-07 拡張が見つからない（閲覧のみ）"); return; }
  try { me = await window.nostr.getPublicKey(); } catch (e) { say("NIP-07: " + e); return; }
  $("who").textContent = me.slice(0, 12) + "…";
  for (const b of document.querySelectorAll(".cmd")) b.disabled = false;
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
$("knock").onclick = () => send({ type: "knock", room: "nostarou-house", message: "こんにちは" });
$("resync").onclick = () => send({ type: "snapshot" });
// talk: plain text, public, up to 280 characters (the town refuses longer)
$("talkForm").onsubmit = (e) => {
  e.preventDefault();
  const text = $("talkText").value.trim();
  if (!text) return;
  if ([...text].length > 280) { say("talk: 280文字まで"); return; }
  send({ type: "talk", text });
  $("talkText").value = "";
};

if (!TOWN || !/^[0-9a-f]{64}$/.test(TOWN) || RELAYS.length === 0) {
  statusEl.textContent = "town pubkey / relays not configured (config.js or ?town=&relays=)";
} else {
  $("town").textContent = TOWN.slice(0, 12) + "…";
  RELAYS.forEach(connect);
  updateStatus();
}
setTimeout(() => { if (window.nostr) login(); }, 500);
requestAnimationFrame(draw);
