// node --test webtest/*.test.js : phone readability, map height and chat attribution (iPhone report).
const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const V = require("../web/viewport.js");
const Chat = require("../web/chatlog.js");

const room = {
  id: "town", width: 58, height: 20,
  houses: [
    { id: "labomi-house", owner: "labomi", rect: { x: 0, y: 0, w: 22, h: 20 } },
    { id: "nostarou-house", owner: "nostarou", rect: { x: 26, y: 0, w: 32, h: 20 } },
  ],
};
const actors = { labomi: { id: "labomi", name: "らぼみ" }, nostarou: { id: "nostarou", name: "のすたろう" } };

test("scroll mode never shows tiles below 24px (phone text was 16px-tile tiny)", () => {
  assert.equal(V.SCROLL_MIN_TILE, 24);
  assert.equal(V.scrollTile(320, 20), 24); // 361x659 iPhone: 320px stage would give 16px
  assert.equal(V.scrollTile(560, 20), 28);
  assert.equal(V.scrollTile(2000, 20), 32);
  // fit / scroll choice is unchanged (it works on the real phone)
  assert.equal(V.chooseMode(345, 58), "scroll");
  assert.equal(V.chooseMode(1424, 58), "fit");
});

test("canvas labels are enlarged when tiles are shown below native size", () => {
  assert.equal(V.labelScale(32), 13 / 11);        // native tile: 11px tag -> >= 13px
  assert.ok(11 * V.labelScale(24) * 24 / 32 >= 13); // 24px tiles: still >= 13 css px
  assert.ok(11 * V.labelScale(16) * 16 / 32 >= 13);
  assert.equal(V.labelScale(0), 1);
});

test("the map stage fills the height down to the chat panel (no empty band)", () => {
  // innerH 659, stage top 161, status+relays 48, chat 244 -> 206
  assert.equal(V.stageHeight(659, 161, 48, 244), 206);
  assert.equal(V.stageHeight(553, 161, 48, 300), 160); // never below the minimum
  // the stage height does not depend on the town height any more
  assert.equal(V.stageHeight(900, 100, 50, 250), 500);
});

test("say lines name the actor; only the viewer's own talk is \"あなた\"", () => {
  const owner = { selfGuestId: "nostr:aaaaaaaaaaaaaaaa", selfActorId: "nostarou" }; // logged in as the town owner
  const say = Chat.entry({ type: "say", actor: { id: "nostarou", name: "のすたろう" }, message: "よっ" }, owner);
  assert.equal(say.who, "のすたろう");
  assert.equal(say.kind, "resident");
  const mine = Chat.entry({ type: "talk", by: "nostr:aaaaaaaaaaaaaaaa", role: "owner", to: "nostarou", message: "hi" }, owner);
  assert.equal(mine.who, "あなた");
  assert.equal(mine.kind, "self");
  const other = Chat.entry({ type: "talk", by: "nostr:cccccccccccccccc", role: "owner", message: "x" }, { selfGuestId: "nostr:bbbbbbbbbbbbbbbb" });
  assert.equal(other.who, "オーナー");
});

test("knock targets: every house, and a house nobody listens at is marked", () => {
  const ts = V.knockTargets(room, actors, ["nostarou"]);
  assert.deepEqual(ts.map(t => [t.id, t.label, t.reachable]),
    [["labomi-house", "らぼみの家", false], ["nostarou-house", "のすたろうの家", true]]);
  const line = Chat.entry({ type: "knock", by: "nostr:bbbbbbbbbbbbbbbb", house: "labomi-house", message: "やあ" }, { houses: ts });
  assert.match(line.text, /らぼみの家/);
  assert.match(line.text, /誰にも届かない/);
  const ok = Chat.entry({ type: "knock", by: "labomi", house: "nostarou-house" }, { houses: ts });
  assert.doesNotMatch(ok.text, /届かない/);
  assert.match(Chat.resultText({ cmd: "move", ok: false, error: "forbidden", role: "owner" }), /招待/);
});

test("the knock button has a house picker, not a fixed nostarou-house", () => {
  const html = fs.readFileSync(path.join(__dirname, "../web/nostr.html"), "utf8");
  const js = fs.readFileSync(path.join(__dirname, "../web/nostr.js"), "utf8");
  assert.match(html, /<select[^>]*id="knockTo"/);
  assert.doesNotMatch(js, /type: "knock", room: "nostarou-house"/);
  // phone text sizes: chat 16px, status / buttons 15px
  const css = html.slice(html.indexOf("<style>"), html.indexOf("</style>"));
  assert.match(css, /body\.narrow #log \{[^}]*font-size:16px/);
  assert.match(css, /body\.narrow #status \{[^}]*font-size:15px/);
  assert.match(css, /body\.narrow button, body\.narrow select \{[^}]*font-size:15px/);
});
