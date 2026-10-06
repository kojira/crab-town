// town-next viewer: the page. Layout and keyboard follow docs/town-next-ui.md
// chapters 1 and 2; every button is one call of the existing town-next API
// (api.ts). Pure decisions live in core.ts (tested under node).
import * as C from "./core";
import { call, worldURL, type Op, type Signer } from "./api";

const $ = <T extends HTMLElement>(id: string) => document.getElementById(id) as T;
const base = location.origin;
const STEP_KEY = "town-next.step";

const st = C.emptyState();
let me = "";             // my pubkey once signed in (window.nostr)
let signer: Signer | undefined;
let step: C.StepName = C.parseStep(localStorage.getItem(STEP_KEY));
let layout: C.Layout = C.layoutFor(innerWidth);
let cam: C.Cam = { x: 0, y: 0 };
let panned = false;      // the user dragged away from "me": stop following
let lift = 0;            // current keyboard lift of the input bar (px)
let follow = true;       // chat log follows new lines
let unread = 0;

const map = $<HTMLCanvasElement>("map");
const mini = $<HTMLCanvasElement>("mini");
const ctx = map.getContext("2d")!;
const mctx = mini.getContext("2d")!;

function tile() { return C.tileFor(layout, step); }
function room() { return C.roomOf(st, me); }
function actors() { return Object.values(st.actors); }

// ---- layout (1.1 / 1.2 / 1.4) ----
function applyLayout() {
  layout = C.layoutFor(innerWidth);
  const s = C.STEPS[step];
  document.body.dataset.layout = layout;
  const r = document.documentElement.style;
  r.setProperty("--map-h", `${s.mapSvh}svh`);
  r.setProperty("--chat-font", `${s.chat}px`);
  r.setProperty("--input-font", `${s.input}px`);
  r.setProperty("--min-lines", String(s.minLines));
  resizeCanvas();
  if (lift) { padChat(); placePeek(); } // the chat box / line height may have changed with the step
}
function resizeCanvas() {
  const wrap = $("mapwrap").getBoundingClientRect();
  const dpr = devicePixelRatio || 1;
  map.width = Math.round(wrap.width * dpr); map.height = Math.round(wrap.height * dpr);
  map.style.width = wrap.width + "px"; map.style.height = wrap.height + "px";
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
  const rm = room();
  if (rm) {
    const sc = C.minimapScale(wrap.width, rm);
    mini.width = Math.round(rm.width * sc * dpr); mini.height = Math.round(rm.height * sc * dpr);
    mini.style.width = rm.width * sc + "px"; mini.style.height = rm.height * sc + "px";
    mctx.setTransform(dpr, 0, 0, dpr, 0, 0);
  }
  recenter(false);
}
function viewSize() { const r = map.getBoundingClientRect(); return { w: r.width, h: r.height }; }
function recenter(force: boolean) {
  const rm = room(); if (!rm) return;
  if (force) panned = false;
  if (panned) { const v = viewSize(); cam = C.clampCam(cam, tile(), v.w, v.h, rm); return; }
  const a = st.actors[me];
  const p = a && !a.hidden ? a.pos : C.defaultFocus(rm);
  const v = viewSize();
  cam = C.centerOn(p, tile(), v.w, v.h, rm);
  $("recenter").hidden = true;
}

// ---- drawing ----
const FLOOR: Record<string, string> = {
  grass: "#6fae5a", wood: "#c89a64", darkwood: "#8a5a36", pinkwood: "#e3a6a6", tile: "#d9d4c7", bathtile: "#e8eef2",
  stone: "#a9a39a", carpet: "#9b7fb0", pinkcarpet: "#efb3c9", bluecarpet: "#8fb6d9", cushionfloor: "#d8cfa8",
  mat: "#b9b27a", rug: "#b06a5a", hall: "#c2a07a", marble: "#e6e2dc",
};
function draw() {
  const rm = room(); const v = viewSize(); const t = tile();
  ctx.fillStyle = "#1b1d24"; ctx.fillRect(0, 0, v.w, v.h);
  if (!rm) { requestAnimationFrame(draw); return; }
  if (!panned) recenter(false);
  ctx.save(); ctx.translate(-cam.x, -cam.y);
  for (const z of rm.zones || []) {
    ctx.fillStyle = FLOOR[z.floor] || "#bbb";
    ctx.fillRect(z.rect.x * t, z.rect.y * t, z.rect.w * t, z.rect.h * t);
  }
  ctx.fillStyle = "#3a3340";
  for (const w of rm.walls || []) ctx.fillRect(w.x * t, w.y * t, w.w * t, w.h * t);
  ctx.fillStyle = "#c9a76a";
  for (const d of rm.doors || []) ctx.fillRect(d.x * t, d.y * t, t, t);
  for (const f of rm.furniture || []) {
    const r = C.furnitureRect(f);
    ctx.fillStyle = f.walkable ? "rgba(255,255,255,.15)" : f.function ? "#5b6fa8" : "#6e6258";
    ctx.fillRect(r.x * t + 2, r.y * t + 2, r.w * t - 4, r.h * t - 4);
    if (f.function) {
      ctx.fillStyle = "#fff"; ctx.font = `${C.STEPS[step].tag}px sans-serif`; ctx.textAlign = "center";
      ctx.fillText(f.label, (r.x + r.w / 2) * t, (r.y + r.h / 2) * t + 4, r.w * t);
    }
  }
  // zones this viewer may not see: frosted (hidden_zones from /world)
  ctx.fillStyle = "rgba(30,30,40,.82)";
  for (const z of rm.zones || []) if (C.isHiddenZone(rm, z.id)) ctx.fillRect(z.rect.x * t, z.rect.y * t, z.rect.w * t, z.rect.h * t);
  for (const a of actors()) {
    if (a.hidden || a.room !== rm.id) continue;
    const cx = (a.pos.x + 0.5) * t, cy = (a.pos.y + 0.5) * t;
    ctx.fillStyle = a.id === me ? "#ffd84a" : "#f2f2f2";
    ctx.beginPath(); ctx.arc(cx, cy, t * 0.36, 0, Math.PI * 2); ctx.fill();
    ctx.strokeStyle = "#222"; ctx.lineWidth = 2; ctx.stroke();
    const tag = C.nameTag(a.name, C.labelOf(st, a.id));
    ctx.font = `${C.STEPS[step].tag}px sans-serif`; ctx.textAlign = "center";
    const w = ctx.measureText(tag).width + 8;
    ctx.fillStyle = "rgba(0,0,0,.65)"; ctx.fillRect(cx - w / 2, cy - t * 0.9 - 12, w, 16);
    ctx.fillStyle = "#fff"; ctx.fillText(tag, cx, cy - t * 0.9);
  }
  ctx.restore();
  drawMini(rm);
  requestAnimationFrame(draw);
}
function drawMini(rm: C.Room) {
  if ($("miniwrap").hidden) return;
  const sc = C.minimapScale(viewSize().w, rm);
  mctx.clearRect(0, 0, rm.width * sc, rm.height * sc);
  for (const z of rm.zones || []) {
    if (C.isHiddenZone(rm, z.id)) continue; // only what /world lets this viewer see
    mctx.fillStyle = FLOOR[z.floor] || "#bbb";
    mctx.fillRect(z.rect.x * sc, z.rect.y * sc, z.rect.w * sc, z.rect.h * sc);
  }
  for (const a of actors()) {
    if (a.hidden || a.room !== rm.id) continue;
    mctx.fillStyle = a.id === me ? "#ffd84a" : "#fff";
    mctx.fillRect(a.pos.x * sc, a.pos.y * sc, Math.max(2, sc), Math.max(2, sc));
  }
  const v = viewSize(), t = tile();
  mctx.strokeStyle = "#fff"; mctx.lineWidth = 1;
  mctx.strokeRect((cam.x / t) * sc, (cam.y / t) * sc, (v.w / t) * sc, (v.h / t) * sc);
}

// ---- chat ----
function addChat(line: C.ChatLine | { system: string }) {
  const log = $("chatlog");
  const div = document.createElement("div");
  div.className = "line";
  if ("system" in line) { div.classList.add("sys"); div.textContent = line.system; }
  else {
    const who = document.createElement("b"); who.textContent = C.nameTag(line.name, line.label) + " ";
    div.append(who, document.createTextNode(line.text));
  }
  log.append(div);
  if (follow) log.scrollTop = log.scrollHeight;
  if (lift) placePeek();
  else { unread++; const p = $("newpill"); p.textContent = `新着 ${unread} 件`; p.hidden = false; }
}
$("chatlog").addEventListener("scroll", () => {
  const log = $("chatlog");
  follow = log.scrollHeight - log.scrollTop - log.clientHeight < 8;
  if (follow) { unread = 0; $("newpill").hidden = true; }
});
$("newpill").addEventListener("click", () => { const l = $("chatlog"); l.scrollTop = l.scrollHeight; });

// ---- API calls ----
async function run(op: Op): Promise<boolean> {
  if (!signer) { addChat({ system: "操作には NIP-07 の署名（window.nostr）が必要" }); return false; }
  try {
    const r = await call(signer, base, op);
    if (!r.ok) addChat({ system: C.errorText(r.status, r.error || "") });
    else C.applyRights(st, r.you, r.rights);
    return r.ok;
  } catch (e) { addChat({ system: "通信できない: " + (e as Error).message }); return false; }
}

// ---- sheet (行動 / furniture / actor / plot) ----
function openSheet(title: string, items: { label: string; disabled?: boolean; act: () => void }[]) {
  $("sheettitle").textContent = title;
  const list = $("sheetlist"); list.replaceChildren();
  for (const it of items) {
    const b = document.createElement("button"); b.type = "button"; b.textContent = it.label; b.disabled = !!it.disabled;
    b.addEventListener("click", () => { closeSheet(); it.act(); });
    list.append(b);
  }
  $("sheet").hidden = false;
}
function closeSheet() { $("sheet").hidden = true; }
$("sheetclose").addEventListener("click", closeSheet);

function useFurniture(f: C.Furniture) { void run({ path: "/interact", body: { furniture: f.id } }); }
function actionSheet() {
  const rm = room();
  const items: { label: string; disabled?: boolean; act: () => void }[] = [];
  if (signer && !st.actors[me]) items.push({ label: "町に入る", act: () => void run({ path: "/join" }) });
  if (rm) for (const a of C.actionsHere(rm, st.actors[me], actors())) {
    if (a.type === "interact") items.push({ label: `${a.furniture.label}を使う${a.busy ? "（使用中）" : ""}`, disabled: a.busy, act: () => useFurniture(a.furniture) });
    else items.push({ label: `区画 ${a.house.id} を申請`, act: () => void run({ path: "/plots/apply", body: { house: a.house.id } }) });
  }
  for (const n of C.STEP_NAMES) items.push({ label: `文字サイズ: ${C.STEP_LABEL[n]}${n === step ? " ✓" : ""}`, act: () => setStep(n) });
  openSheet("行動", items);
}
$("actbtn").addEventListener("click", actionSheet);
function setStep(n: C.StepName) { step = n; localStorage.setItem(STEP_KEY, n); applyLayout(); }

function onTap(p: C.Pos) {
  const rm = room(); if (!rm) return;
  const h = C.hitAt(rm, actors(), p);
  switch (h.kind) {
    case "tile": void run({ path: "/move", body: p }); break;
    case "furniture": openSheet(h.furniture.label, [{ label: "使う", act: () => useFurniture(h.furniture) }]); break;
    case "actor": openSheet(C.nameTag(h.actor.name, C.labelOf(st, h.actor.id)), []); break;
    case "plot": openSheet(`空き地 ${h.house.id}`, [{ label: "この区画を申請", act: () => void run({ path: "/plots/apply", body: { house: h.house.id } }) }]); break;
  }
}

// ---- map input: tap = walk / open, drag = pan; the browser keeps pinch ----
let down: { x: number; y: number; cx: number; cy: number; id: number } | null = null;
let dragging = false;
map.addEventListener("pointerdown", (e) => { down = { x: e.clientX, y: e.clientY, cx: cam.x, cy: cam.y, id: e.pointerId }; dragging = false; });
map.addEventListener("pointermove", (e) => {
  if (!down || e.pointerId !== down.id) return;
  const dx = e.clientX - down.x, dy = e.clientY - down.y;
  if (!dragging && !C.isDrag(dx, dy)) return;
  dragging = true; panned = true; $("recenter").hidden = false;
  const rm = room(); if (!rm) return;
  const v = viewSize();
  cam = C.clampCam({ x: down.cx - dx, y: down.cy - dy }, tile(), v.w, v.h, rm);
});
map.addEventListener("pointerup", (e) => {
  if (!down) return;
  if (!dragging) { const r = map.getBoundingClientRect(); onTap(C.screenToTile(cam, e.clientX - r.left, e.clientY - r.top, tile())); }
  down = null;
});
map.addEventListener("pointercancel", () => { down = null; });
$("recenter").addEventListener("click", () => recenter(true));
mini.addEventListener("click", (e) => {
  const rm = room(); if (!rm) return;
  const r = mini.getBoundingClientRect();
  const p = C.minimapToTile(e.clientX - r.left, e.clientY - r.top, C.minimapScale(viewSize().w, rm), rm);
  const v = viewSize(); cam = C.centerOn(p, tile(), v.w, v.h, rm); panned = true; $("recenter").hidden = false;
});
$("minifold").addEventListener("click", () => { $("miniwrap").hidden = true; $("miniopen").hidden = false; });
$("miniopen").addEventListener("click", () => { $("miniwrap").hidden = false; $("miniopen").hidden = true; });

// PC: arrows / WASD = one tile (/move once), Enter = focus the chat input.
const STEP_KEYS: Record<string, [number, number]> = {
  ArrowUp: [0, -1], ArrowDown: [0, 1], ArrowLeft: [-1, 0], ArrowRight: [1, 0], w: [0, -1], s: [0, 1], a: [-1, 0], d: [1, 0],
};
addEventListener("keydown", (e) => {
  if (document.activeElement === $("say")) return;
  if (e.key === "Enter") { e.preventDefault(); $("say").focus(); return; }
  const k = STEP_KEYS[e.key]; const a = st.actors[me];
  if (k && a && !a.hidden) { e.preventDefault(); void run({ path: "/move", body: { x: a.pos.x + k[0], y: a.pos.y + k[1] } }); }
});

// ---- say ----
$("inputbar").addEventListener("submit", async (e) => {
  e.preventDefault();
  const inp = $<HTMLInputElement>("say");
  const text = inp.value.trim(); if (!text) return;
  if (await run({ path: "/say", body: { text } })) inp.value = "";
});

// ---- 2: keyboard. Only the input bar's transform is written. ----
function onViewport() {
  const next = C.keyboardLift(innerHeight, window.visualViewport);
  if (next === null || !C.needsWrite(lift, next)) return;
  lift = next;
  $("inputbar").style.transform = lift ? `translateY(${-lift}px)` : "";
  padChat();
  placePeek();
}
// rule 6: the newest lines right above the lifted bar, inside visualViewport
function placePeek() {
  const peek = $("peek"), vv = window.visualViewport;
  const log = $("chatlog"), lcs = getComputedStyle(log), pcs = getComputedStyle(peek);
  const p = lift && vv ? C.chatPeek(
    { height: vv.height, offsetTop: vv.offsetTop, scale: vv.scale },
    $("inputbar").offsetHeight, $("chat").getBoundingClientRect().top, parseFloat(lcs.paddingTop),
    parseFloat(pcs.lineHeight), parseFloat(pcs.paddingTop), lift) : null;
  if (!p) { peek.hidden = true; peek.replaceChildren(); return; }
  const src = Array.from(log.querySelectorAll(".line")).slice(-p.lines);
  if (src.length === 0) { peek.hidden = true; peek.replaceChildren(); return; }
  peek.replaceChildren(...src.map((l) => l.cloneNode(true)));
  peek.style.top = `${p.top + (p.lines - src.length) * parseFloat(pcs.lineHeight)}px`;
  peek.hidden = false;
}
// rule 5: room = the log's box (= #chat, never resized) minus its top padding
function padChat() {
  const log = $("chatlog"), cs = getComputedStyle(log);
  const room = $("chat").clientHeight - parseFloat(cs.paddingTop);
  const pad = lift ? C.chatPad(lift, room, parseFloat(cs.lineHeight)) : 0;
  log.style.paddingBottom = pad ? `${pad}px` : "";
  if (follow) log.scrollTop = log.scrollHeight;
}
window.visualViewport?.addEventListener("resize", onViewport);
window.visualViewport?.addEventListener("scroll", onViewport);

// ---- /world ----
let ws: WebSocket | null = null;
async function connect() {
  ws?.close();
  const url = await worldURL(base, signer);
  ws = new WebSocket(url);
  ws.onmessage = (e) => {
    let m: C.WorldMsg; try { m = JSON.parse(e.data); } catch { return; }
    const firstRoom = st.rooms.length === 0;
    const line = C.applyMsg(st, m);
    if (line) addChat(line);
    if (m.type === "snapshot" && firstRoom) resizeCanvas();
  };
  ws.onclose = () => { setTimeout(() => void connect(), 3000); ws = null; };
}

async function signIn() {
  const n = (window as any).nostr as Signer | undefined;
  if (!n) { addChat({ system: "window.nostr が無いので見るだけ（NIP-07 拡張で操作できる）" }); return; }
  try { me = await n.getPublicKey(); signer = n; $("login").hidden = true; await connect(); }
  catch (e) { addChat({ system: "署名できない: " + (e as Error).message }); }
}
$("login").addEventListener("click", () => void signIn());

let lastW = innerWidth;
addEventListener("resize", () => { if (innerWidth !== lastW) { lastW = innerWidth; applyLayout(); } });
// read-only handle for tests and headless screenshots (docs/town-next-ui.md 4)
(window as any).__townNext = {
  state: st,
  tileToScreen: (x: number, y: number) => ({ x: (x + 0.5) * tile() - cam.x, y: (y + 0.5) * tile() - cam.y }),
};
applyLayout();
void connect();
requestAnimationFrame(draw);
