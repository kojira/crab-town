// town-next viewer: pure logic (no DOM, no network). Bundled into
// internal/nextweb/static/app.js by scripts/build-next-web.mjs; the node tests
// (webtest-next/*.test.js) load this file directly through esbuild.
//
// Design: docs/town-next-ui.md chapters 1 (layout) and 2 (keyboard). Nothing
// here invents a value the API does not send (0.4): what /world does not
// carry is not shown.

// ---- shapes of the /world JSON (internal/world + internal/next View) ----
export interface Pos { x: number; y: number }
export interface Size { w: number; h: number }
export interface Rect { x: number; y: number; w: number; h: number }
export interface Zone { id: string; name: string; floor: string; visibility: string; rect: Rect; house?: string; private?: boolean }
export interface House { id: string; owner: string; invited?: string[] | null; rect: Rect }
export interface Furniture {
  id: string; kind: string; label: string; function: string;
  pos: Pos; size?: Size; access?: Pos; state?: string; walkable?: boolean;
}
export interface Room {
  id: string; width: number; height: number;
  houses?: House[] | null; zones?: Zone[] | null; walls?: Rect[] | null; doors?: Pos[] | null;
  furniture?: Furniture[] | null; hidden_zones?: string[] | null; in_use?: string[] | null;
}
export interface Actor { id: string; name: string; room: string; pos: Pos; state: string; using?: string; target?: Pos; hidden?: boolean }
export interface Rights { town_owner?: boolean; holds?: string[]; invited?: string[]; label?: string }
export interface WorldMsg {
  type: string; viewer?: string; rooms?: Room[]; actors?: Actor[]; rights?: Record<string, Rights>;
  room?: string; actor?: Actor; message?: string; in_use?: string[]; hidden_zones?: string[];
}

// ---- 1.1 / 1.2: which layout ----
export type Layout = "phone" | "pc";
export const PC_MIN_WIDTH = 1024; // 1023px and below use the phone portrait layout
export function layoutFor(width: number): Layout {
  return width >= PC_MIN_WIDTH ? "pc" : "phone";
}

// ---- 1.4: text size steps (saved per device, never sent to the server) ----
export type StepName = "small" | "medium" | "large";
export interface TextStep { chat: number; tag: number; tile: number; mapSvh: number; minLines: number; input: number }
export const STEPS: Record<StepName, TextStep> = {
  small: { chat: 14, tag: 11, tile: 24, mapSvh: 60, minLines: 4, input: 16 },
  medium: { chat: 16, tag: 12, tile: 28, mapSvh: 58, minLines: 4, input: 16 },
  large: { chat: 18, tag: 14, tile: 32, mapSvh: 55, minLines: 3, input: 18 },
};
export const STEP_NAMES: StepName[] = ["small", "medium", "large"];
export const STEP_LABEL: Record<StepName, string> = { small: "小", medium: "中", large: "大" };
export const MIN_TILE = 24; // never draw a tile smaller than this (1.1)
export const PC_TILE = 32;  // PC default tile (1.2)
export function parseStep(s: string | null | undefined): StepName {
  return s === "small" || s === "large" ? s : "medium";
}
export function tileFor(layout: Layout, step: StepName): number {
  const t = STEPS[step].tile;
  return Math.max(MIN_TILE, layout === "pc" ? Math.max(t, PC_TILE) : t);
}

// ---- 2: keyboard. Lift only the input bar, by
//   innerHeight - (visualViewport.height + visualViewport.offsetTop)
// null = do nothing (pinch / focus zoom: scale != 1).
export interface VV { height: number; offsetTop: number; scale: number }
export function keyboardLift(innerHeight: number, vv: VV | null | undefined): number | null {
  if (!vv) return 0;
  if (vv.scale !== 1) return null;
  return Math.max(0, Math.round(innerHeight - (vv.height + vv.offsetTop)));
}
// Rule 5: the chat log's inner bottom padding while the bar is lifted. The
// lifted bar covers `lift` px of the chat box from below, but the log is
// box-sizing: border-box, so padding larger than the room it has (box height
// minus its top padding) would grow the log past #chat and the follow scroll
// would push the lines above the top of the chat, under the map. Keep at
// least one line of room: pad = min(lift, room - lineH), never below 0.
export function chatPad(lift: number, room: number, lineH: number): number {
  return Math.max(0, Math.min(lift, Math.floor(room - lineH)));
}
// Rule 3: a change of 1px or less is not written.
export function needsWrite(prev: number, next: number): boolean {
  return Math.abs(next - prev) > 1;
}

// ---- camera (px of the whole town; top-left of what is on screen) ----
export interface Cam { x: number; y: number }
export function clampCam(c: Cam, tile: number, viewW: number, viewH: number, room: Room): Cam {
  const axis = (v: number, view: number, cells: number) => {
    const max = cells * tile - view;
    return max <= 0 ? max / 2 : Math.min(Math.max(v, 0), max); // smaller than the view: centre it
  };
  return { x: axis(c.x, viewW, room.width), y: axis(c.y, viewH, room.height) };
}
export function centerOn(p: Pos, tile: number, viewW: number, viewH: number, room: Room): Cam {
  return clampCam({ x: (p.x + 0.5) * tile - viewW / 2, y: (p.y + 0.5) * tile - viewH / 2 }, tile, viewW, viewH, room);
}
export function screenToTile(cam: Cam, px: number, py: number, tile: number): Pos {
  return { x: Math.floor((px + cam.x) / tile), y: Math.floor((py + cam.y) / tile) };
}
// a tap that moved further than this is a drag (pan), not a tap
export const DRAG_PX = 8;
export function isDrag(dx: number, dy: number): boolean {
  return Math.hypot(dx, dy) > DRAG_PX;
}
// The starting point when nobody is "me" yet: the outdoor zone (a garden), or the middle.
export function defaultFocus(room: Room): Pos {
  const out = (room.zones || []).find((z) => !z.house);
  if (out) return { x: out.rect.x + Math.floor(out.rect.w / 2), y: out.rect.y + Math.floor(out.rect.h / 2) };
  return { x: Math.floor(room.width / 2), y: Math.floor(room.height / 2) };
}

// ---- minimap (overlaid at the top right, 30% of the map width) ----
export const MINIMAP_RATIO = 0.3;
export function minimapScale(mapW: number, room: Room): number {
  return (mapW * MINIMAP_RATIO) / room.width;
}
export function minimapToTile(mx: number, my: number, scale: number, room: Room): Pos {
  const c = (v: number, n: number) => Math.min(n - 1, Math.max(0, Math.floor(v / scale)));
  return { x: c(mx, room.width), y: c(my, room.height) };
}

// ---- geometry of a room ----
export const inRect = (r: Rect, p: Pos) => p.x >= r.x && p.y >= r.y && p.x < r.x + r.w && p.y < r.y + r.h;
export function furnitureRect(f: Furniture): Rect {
  return { x: f.pos.x, y: f.pos.y, w: f.size?.w || 1, h: f.size?.h || 1 };
}
export function zoneAt(room: Room, p: Pos): Zone | undefined {
  return (room.zones || []).find((z) => inRect(z.rect, p));
}
export function houseAt(room: Room, p: Pos): House | undefined {
  return (room.houses || []).find((h) => inRect(h.rect, p));
}
export function isHiddenZone(room: Room, zoneId: string | undefined): boolean {
  return !!zoneId && (room.hidden_zones || []).includes(zoneId);
}
export function isWall(room: Room, p: Pos): boolean {
  if ((room.doors || []).some((d) => d.x === p.x && d.y === p.y)) return false;
  return (room.walls || []).some((w) => inRect(w, p));
}
// A vacant plot is a house with no holder (internal/next: owner "").
export function vacantPlots(room: Room): House[] {
  return (room.houses || []).filter((h) => h.owner === "");
}

// ---- what a tap on a tile means (3.1) ----
export type Hit =
  | { kind: "actor"; actor: Actor }
  | { kind: "furniture"; furniture: Furniture }
  | { kind: "plot"; house: House }
  | { kind: "tile"; pos: Pos }
  | { kind: "none" };
export function hitAt(room: Room, actors: Actor[], p: Pos): Hit {
  if (p.x < 0 || p.y < 0 || p.x >= room.width || p.y >= room.height) return { kind: "none" };
  const a = actors.find((a) => !a.hidden && a.room === room.id && a.pos.x === p.x && a.pos.y === p.y);
  if (a) return { kind: "actor", actor: a };
  const f = (room.furniture || []).find((f) => inRect(furnitureRect(f), p));
  const hidden = isHiddenZone(room, zoneAt(room, p)?.id);
  if (f && f.function && !hidden) return { kind: "furniture", furniture: f };
  const h = houseAt(room, p);
  if (h && h.owner === "") return { kind: "plot", house: h };
  if (f && !f.walkable) return { kind: "none" }; // decoration: blocks walking, nothing to do
  if (isWall(room, p)) return { kind: "none" };
  return { kind: "tile", pos: p };
}

// ---- the [行動] list: what can be done from here, with existing APIs only ----
export type Action =
  | { type: "interact"; furniture: Furniture; busy: boolean }
  | { type: "apply"; house: House };
export function actionsHere(room: Room, me: Actor | undefined, actors: Actor[]): Action[] {
  const out: Action[] = [];
  if (me && !me.hidden && me.room === room.id) {
    const here = zoneAt(room, me.pos);
    for (const f of room.furniture || []) {
      if (!f.function || !f.access) continue;
      const z = zoneAt(room, f.access);
      if (!here || !z || z.id !== here.id || isHiddenZone(room, z.id)) continue;
      out.push({ type: "interact", furniture: f, busy: actors.some((o) => o.id !== me.id && o.using === f.id) });
    }
  }
  for (const h of vacantPlots(room)) out.push({ type: "apply", house: h });
  return out;
}

// ---- API errors: the status, said short (3.1) ----
export function errorText(status: number, msg: string): string {
  const m = (msg || "").toLowerCase();
  let s: string;
  if (status === 409) {
    if (m.includes("tile")) s = "そこは他の人がいる";
    else if (m.includes("in use")) s = "使用中";
    else if (m.includes("join")) s = "まだ町に入っていない";
    else if (m.includes("vacant")) s = "空き地ではない";
    else if (m.includes("full")) s = "町が満員";
    else s = "いまはできない";
  } else if (status === 403) s = "入れない";
  else if (status === 401) s = "署名が通らない";
  else if (status === 404) s = "見つからない";
  else s = "できない";
  return `${s}（${status}）`;
}

// ---- the world as this viewer sees it, fed by /world messages ----
export interface ChatLine { by: string; name: string; label: string; text: string }
export interface WorldState {
  viewer: string;
  rooms: Room[];
  actors: Record<string, Actor>;
  rights: Record<string, Rights>;
}
export function emptyState(): WorldState {
  return { viewer: "", rooms: [], actors: {}, rights: {} };
}
export function labelOf(s: WorldState, id: string): string {
  return s.rights[id]?.label || "";
}
// applyMsg updates s and returns a chat line for a say message (else null).
export function applyMsg(s: WorldState, m: WorldMsg): ChatLine | null {
  switch (m.type) {
    case "snapshot":
      s.viewer = m.viewer || "";
      s.rooms = m.rooms || [];
      s.rights = m.rights || {};
      s.actors = {};
      for (const a of m.actors || []) s.actors[a.id] = a;
      return null;
    case "actor":
      if (m.actor) s.actors[m.actor.id] = m.actor;
      return null;
    case "leave":
      if (m.actor) delete s.actors[m.actor.id];
      return null;
    case "occupancy": {
      const r = s.rooms.find((r) => r.id === m.room);
      if (r) { r.in_use = m.in_use || []; r.hidden_zones = m.hidden_zones || []; }
      return null;
    }
    case "say": {
      if (!m.message || !m.actor) return null;
      s.actors[m.actor.id] = m.actor;
      return { by: m.actor.id, name: m.actor.name, label: labelOf(s, m.actor.id), text: m.message };
    }
  }
  return null; // knock / interact / others: no 知らせ yet (docs/town-next-ui.md 5-9)
}
// applyRights: a successful call answers {you, rights}; keep that label (it is
// what the server said). Other actors' labels come only with a snapshot.
export function applyRights(s: WorldState, you: string | undefined, r: Rights | undefined): void {
  if (!you || !r) return;
  if (r.label) s.rights[you] = r; else delete s.rights[you];
}
export function roomOf(s: WorldState, me: string): Room | undefined {
  const a = s.actors[me];
  return (a && s.rooms.find((r) => r.id === a.room)) || s.rooms[0];
}
export function nameTag(name: string, label: string): string {
  return label ? `${name} [${label}]` : name;
}
