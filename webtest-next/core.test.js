// node --test webtest-next/ : the pure logic of the town-next viewer
// (web-next-src/core.ts) -- layout, text steps, keyboard lift, taps, actions.
const test = require("node:test");
const assert = require("node:assert/strict");
const C = require("./load")("core");

// a small town like internal/next's testData: a held house, a vacant plot, a garden
const room = () => ({
  id: "town", width: 12, height: 6,
  houses: [
    { id: "a-house", owner: "aa", invited: [], rect: { x: 0, y: 0, w: 6, h: 6 } },
    { id: "plot", owner: "", invited: [], rect: { x: 7, y: 0, w: 5, h: 6 } },
  ],
  zones: [
    { id: "a-room", name: "部屋", floor: "wood", visibility: "owner", house: "a-house", rect: { x: 1, y: 1, w: 4, h: 4 } },
    { id: "garden", name: "庭", floor: "grass", visibility: "public", rect: { x: 6, y: 0, w: 1, h: 6 } },
    { id: "p-room", name: "空き", floor: "wood", visibility: "public", house: "plot", rect: { x: 8, y: 1, w: 3, h: 4 } },
  ],
  walls: [{ x: 0, y: 0, w: 6, h: 1 }, { x: 5, y: 0, w: 1, h: 6 }, { x: 7, y: 0, w: 1, h: 6 }],
  doors: [{ x: 5, y: 2 }, { x: 7, y: 2 }],
  furniture: [
    { id: "bench", kind: "sofa", label: "ベンチ", function: "visitors", pos: { x: 6, y: 0 }, size: { w: 1, h: 1 }, access: { x: 6, y: 1 }, state: "talking" },
    { id: "bed", kind: "bed", label: "ベッド", function: "standby", pos: { x: 1, y: 3 }, size: { w: 2, h: 2 }, access: { x: 3, y: 3 }, state: "away" },
    { id: "tree", kind: "tree", label: "木", function: "", pos: { x: 6, y: 5 }, size: { w: 1, h: 1 }, access: { x: 0, y: 0 } },
    { id: "rug", kind: "rug", label: "ラグ", function: "", pos: { x: 6, y: 4 }, size: { w: 1, h: 1 }, access: { x: 0, y: 0 }, walkable: true },
  ],
});

test("1023px and below is the phone layout, 1024px and up is PC (1.2)", () => {
  assert.equal(C.layoutFor(390), "phone");
  assert.equal(C.layoutFor(1023), "phone");
  assert.equal(C.layoutFor(1024), "pc");
  assert.equal(C.layoutFor(1280), "pc");
});

test("text steps match the table in 1.4; inputs never under 16px; tiles never under 24px", () => {
  assert.deepEqual(C.STEPS.small, { chat: 14, tag: 11, tile: 24, mapSvh: 60, minLines: 4, input: 16 });
  assert.deepEqual(C.STEPS.medium, { chat: 16, tag: 12, tile: 28, mapSvh: 58, minLines: 4, input: 16 });
  assert.deepEqual(C.STEPS.large, { chat: 18, tag: 14, tile: 32, mapSvh: 55, minLines: 3, input: 18 });
  for (const n of C.STEP_NAMES) {
    assert.ok(C.STEPS[n].input >= 16, n);
    assert.ok(C.STEPS[n].mapSvh >= 55 && C.STEPS[n].mapSvh <= 60, n);
    for (const l of ["phone", "pc"]) assert.ok(C.tileFor(l, n) >= 24, l + n);
  }
  assert.equal(C.tileFor("phone", "medium"), 28);
  assert.equal(C.tileFor("pc", "medium"), 32); // PC default 32px
  assert.equal(C.parseStep("large"), "large");
  assert.equal(C.parseStep("huge"), "medium");
  assert.equal(C.parseStep(null), "medium");
});

test("keyboard lift = innerHeight - (vv.height + vv.offsetTop), never negative, nothing while zoomed (2)", () => {
  assert.equal(C.keyboardLift(844, { height: 844, offsetTop: 0, scale: 1 }), 0);
  assert.equal(C.keyboardLift(844, { height: 508, offsetTop: 0, scale: 1 }), 336);
  assert.equal(C.keyboardLift(844, { height: 508, offsetTop: 100, scale: 1 }), 236); // Safari panned the visual viewport
  assert.equal(C.keyboardLift(844, { height: 900, offsetTop: 0, scale: 1 }), 0);
  assert.equal(C.keyboardLift(844, { height: 400, offsetTop: 0, scale: 1.5 }), null); // pinch / focus zoom: do nothing
  assert.equal(C.keyboardLift(844, null), 0);
  assert.equal(C.needsWrite(336, 337), false);
  assert.equal(C.needsWrite(336, 338), true);
  assert.equal(C.needsWrite(0, 336), true);
});

test("camera: clamped to the town, centred when the town is smaller than the view", () => {
  const r = { id: "town", width: 58, height: 20 };
  const c = C.centerOn({ x: 48, y: 3 }, 28, 390, 489, r);
  assert.equal(c.x, 48.5 * 28 - 195);
  assert.equal(c.y, 0); // clamped at the top
  assert.deepEqual(C.clampCam({ x: 99999, y: -5 }, 28, 390, 489, r), { x: 58 * 28 - 390, y: 0 });
  assert.equal(C.clampCam({ x: 0, y: 0 }, 32, 2000, 489, r).x, (58 * 32 - 2000) / 2);
  assert.deepEqual(C.screenToTile({ x: 100, y: 0 }, 10, 30, 28), { x: 3, y: 1 });
  assert.equal(C.isDrag(5, 5), false);
  assert.equal(C.isDrag(9, 0), true);
});

test("minimap is 30% of the map width and maps back to tiles inside the town", () => {
  const r = { width: 58, height: 20 };
  const s = C.minimapScale(390, r);
  assert.ok(Math.abs(s * 58 - 117) < 1e-9);
  assert.deepEqual(C.minimapToTile(0, 0, s, r), { x: 0, y: 0 });
  assert.deepEqual(C.minimapToTile(9999, 9999, s, r), { x: 57, y: 19 });
});

test("a tap: actor, usable furniture, vacant plot, walkable tile; walls and decorations do nothing", () => {
  const rm = room();
  const actors = [{ id: "bb", name: "bb", room: "town", pos: { x: 6, y: 3 }, state: "idle" }];
  assert.equal(C.hitAt(rm, actors, { x: 6, y: 3 }).kind, "actor");
  assert.equal(C.hitAt(rm, actors, { x: 6, y: 0 }).furniture.id, "bench");
  assert.equal(C.hitAt(rm, actors, { x: 9, y: 2 }).house.id, "plot");
  assert.deepEqual(C.hitAt(rm, actors, { x: 6, y: 2 }), { kind: "tile", pos: { x: 6, y: 2 } });
  assert.deepEqual(C.hitAt(rm, actors, { x: 6, y: 4 }), { kind: "tile", pos: { x: 6, y: 4 } }); // a rug is walked on
  assert.equal(C.hitAt(rm, actors, { x: 6, y: 5 }).kind, "none"); // tree
  assert.equal(C.hitAt(rm, actors, { x: 5, y: 1 }).kind, "none"); // wall
  assert.equal(C.hitAt(rm, actors, { x: 5, y: 2 }).kind, "tile"); // door
  assert.equal(C.hitAt(rm, actors, { x: 12, y: 0 }).kind, "none"); // outside
  // a hidden actor is not hit; furniture in a zone this viewer may not see is not offered
  assert.equal(C.hitAt(rm, [{ ...actors[0], hidden: true }], { x: 6, y: 3 }).kind, "tile");
  rm.hidden_zones = ["a-room"];
  assert.notEqual(C.hitAt(rm, actors, { x: 1, y: 3 }).kind, "furniture");
});

test("[行動]: furniture used from my zone (busy when someone else uses it) and vacant plots", () => {
  const rm = room();
  const me = { id: "me", name: "me", room: "town", pos: { x: 6, y: 2 }, state: "idle" };
  let acts = C.actionsHere(rm, me, [me]);
  assert.deepEqual(acts.map((a) => a.type === "interact" ? "use:" + a.furniture.id : "apply:" + a.house.id), ["use:bench", "apply:plot"]);
  assert.equal(acts[0].busy, false);
  acts = C.actionsHere(rm, me, [me, { id: "x", name: "x", room: "town", pos: { x: 6, y: 1 }, state: "talking", using: "bench" }]);
  assert.equal(acts[0].busy, true);
  // not joined: only the plots
  assert.deepEqual(C.actionsHere(rm, undefined, []).map((a) => a.type), ["apply"]);
});

test("API errors are said short with the status (3.1)", () => {
  assert.equal(C.errorText(409, "tile is taken"), "そこは他の人がいる（409）");
  assert.equal(C.errorText(409, "furniture is in use by someone else"), "使用中（409）");
  assert.equal(C.errorText(409, "join first"), "まだ町に入っていない（409）");
  assert.equal(C.errorText(403, "forbidden"), "入れない（403）");
  assert.equal(C.errorText(401, "NIP-98: event already used"), "署名が通らない（401）");
});

test("world messages: snapshot, actor, leave, occupancy, say (with the rights label)", () => {
  const s = C.emptyState();
  const a = { id: "aa", name: "のすたろう", room: "town", pos: { x: 1, y: 1 }, state: "idle" };
  assert.equal(C.applyMsg(s, { type: "snapshot", viewer: "aa", rooms: [room()], actors: [a], rights: { aa: { holds: ["a-house"], label: "家主" } } }), null);
  assert.equal(s.viewer, "aa");
  assert.equal(C.roomOf(s, "aa").id, "town");
  C.applyMsg(s, { type: "actor", actor: { ...a, pos: { x: 2, y: 1 } } });
  assert.deepEqual(s.actors.aa.pos, { x: 2, y: 1 });
  const line = C.applyMsg(s, { type: "say", actor: a, message: "やあ" });
  assert.deepEqual(line, { by: "aa", name: "のすたろう", label: "家主", text: "やあ" });
  assert.equal(C.nameTag(line.name, line.label), "のすたろう [家主]");
  assert.equal(C.nameTag("bb", ""), "bb"); // no label, nothing added (no AI / human marks)
  C.applyMsg(s, { type: "occupancy", room: "town", in_use: ["a-room"], hidden_zones: ["a-room"] });
  assert.deepEqual(s.rooms[0].hidden_zones, ["a-room"]);
  C.applyMsg(s, { type: "leave", actor: a });
  assert.equal(s.actors.aa, undefined);
  assert.equal(C.applyMsg(s, { type: "knock", by: "x", house: "a-house", message: "hi" }), null); // 知らせ: not yet (5-9)
});

test("the starting focus without me is the outdoor zone", () => {
  assert.deepEqual(C.defaultFocus(room()), { x: 6, y: 3 });
  assert.deepEqual(C.defaultFocus({ id: "r", width: 10, height: 4, zones: [] }), { x: 5, y: 2 });
});

test("the rights in an API answer set my label (nothing else invents one)", () => {
  const s = C.emptyState();
  C.applyRights(s, "aa", { holds: ["a-house"], label: "家主" });
  assert.equal(C.labelOf(s, "aa"), "家主");
  C.applyRights(s, "aa", {});
  assert.equal(C.labelOf(s, "aa"), "");
  C.applyRights(s, undefined, { label: "x" });
  assert.deepEqual(s.rights, {});
});
