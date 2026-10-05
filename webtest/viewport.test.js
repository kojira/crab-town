// node --test webtest/*.test.js : viewport math and chat-log attribution of the Pages viewer.
const test = require("node:test");
const assert = require("node:assert/strict");
const V = require("../web/viewport.js");
const Chat = require("../web/chatlog.js");

// A town shaped like internal/world/town.go (58x20, two houses, a garden).
const room = {
  id: "town", width: 58, height: 20,
  houses: [
    { id: "labomi-house", owner: "labomi", rect: { x: 0, y: 0, w: 22, h: 20 } },
    { id: "nostarou-house", owner: "nostarou", rect: { x: 26, y: 0, w: 32, h: 20 } },
  ],
  zones: [
    { id: "labomi-room", name: "らぼみの部屋", visibility: "owner", house: "labomi-house", rect: { x: 1, y: 13, w: 10, h: 6 } },
    { id: "garden", name: "庭", visibility: "public", rect: { x: 22, y: 0, w: 4, h: 20 } },
  ],
};
const actors = {
  labomi: { id: "labomi", name: "らぼみ", room: "town", pos: { x: 8, y: 6 } },
  nostarou: { id: "nostarou", name: "のすたろう", room: "town", pos: { x: 48, y: 3 } },
};

test("phone portrait scrolls, desktop landscape fits the whole town", () => {
  assert.equal(V.chooseMode(374, 58), "scroll"); // 390px phone
  assert.equal(V.chooseMode(1424, 58), "fit");   // 1440px desktop: 24px a tile
  assert.equal(V.chooseMode(58 * 16, 58), "fit"); // exactly 16px a tile is readable
  assert.equal(V.chooseMode(58 * 16 - 1, 58), "scroll");
});

test("scroll-mode tiles are at least 16px and at most native 32px", () => {
  assert.equal(V.tileSize(200, 20), 16);
  assert.equal(V.tileSize(500, 20), 25);
  assert.equal(V.tileSize(5000, 20), 32);
});

test("view offsets are clamped to the content", () => {
  assert.deepEqual(V.clampView(-50, -50, 374, 400, 1160, 400), { x: 0, y: 0 });
  assert.deepEqual(V.clampView(5000, 0, 374, 400, 1160, 400), { x: 786, y: 0 });
  // content narrower than the view: centred
  assert.deepEqual(V.clampView(10, 10, 400, 300, 200, 300), { x: -100, y: 0 });
});

test("centerOn puts the tile in the middle of the view", () => {
  // tile (24,10) at 20px: centre (490,210), view 374x200 -> (303,110)
  assert.deepEqual(V.centerOn(24, 10, 20, 374, 200, 1160, 400), { x: 303, y: 110 });
  // near the right edge it clamps
  assert.deepEqual(V.centerOn(57, 0, 20, 374, 200, 1160, 400), { x: 786, y: 0 });
});

test("initial focus: self, else the garden (guest / unknown / hidden)", () => {
  assert.deepEqual(V.initialFocus(room, actors, "nostarou"), { x: 48, y: 3, from: "self" });
  assert.deepEqual(V.initialFocus(room, actors, null), { x: 23.5, y: 9.5, from: "garden" });
  assert.deepEqual(V.initialFocus(room, actors, "nostr:abcd"), { x: 23.5, y: 9.5, from: "garden" });
  const hidden = { ...actors, nostarou: { ...actors.nostarou, hidden: true } };
  assert.equal(V.initialFocus(room, hidden, "nostarou").from, "garden");
  assert.equal(V.initialFocus({ ...room, zones: [] }, actors, null).from, "town");
});

test("jump targets come from the layout (houses by owner name, public outdoor zones)", () => {
  const ts = V.jumpTargets(room, actors, "nostarou");
  assert.deepEqual(ts.map(t => t.label), ["自分", "らぼみの家", "のすたろうの家", "庭"]);
  assert.deepEqual(ts.map(t => t.id), ["self", "labomi-house", "nostarou-house", "garden"]);
  assert.deepEqual(V.jumpTargets(room, actors, null).map(t => t.id), ["labomi-house", "nostarou-house", "garden"]);
  assert.deepEqual(V.jumpTargets(null, actors, null), []);
});

test("minimap scale, frame and tap -> tile", () => {
  const s = V.minimapScale(58, 20, 140, 60);
  assert.equal(s, 2);
  assert.deepEqual(V.minimapFrame({ x: 303, y: 0 }, 20, 374, 200, 1160, 400, s), { x: 30.3, y: 0, w: 37.4, h: 20 });
  assert.deepEqual(V.minimapToTile(47, 7, s), { x: 23, y: 3 });
});

test("chat lines say who spoke: resident / guest / owner / self", () => {
  const ctx = { selfGuestId: "nostr:aaaaaaaaaaaaaaaa", selfActorId: null };
  assert.deepEqual(Chat.entry({ type: "say", actor: { id: "nostarou", name: "のすたろう" }, message: "よっ" }, ctx),
    { kind: "resident", who: "のすたろう", actor: "nostarou", text: "よっ" });
  assert.equal(Chat.entry({ type: "talk", by: "nostr:bbbbbbbbbbbbbbbb", role: "guest", to: "nostarou", message: "hi" }, ctx).kind, "guest");
  assert.equal(Chat.entry({ type: "talk", by: "nostr:bbbbbbbbbbbbbbbb", role: "guest", message: "hi" }, ctx).who, "来客 bbbbbbbb");
  assert.equal(Chat.entry({ type: "talk", by: "nostr:aaaaaaaaaaaaaaaa", role: "guest", message: "me" }, ctx).kind, "self");
  assert.equal(Chat.entry({ type: "talk", by: "nostr:cccccccccccccccc", role: "owner", message: "x" }, ctx).kind, "owner");
  const own = Chat.entry({ type: "say", actor: { id: "nostarou", name: "のすたろう" }, message: "x" }, { selfActorId: "nostarou" });
  assert.equal(own.kind, "self");
  assert.equal(Chat.entry({ type: "knock", by: "labomi", house: "nostarou-house" }, ctx).kind, "system");
  assert.equal(Chat.entry({ type: "result" }, ctx), null);
});
