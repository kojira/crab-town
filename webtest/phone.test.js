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

test("phone map takes a share of the column; the chat panel gets the rest", () => {
  // 664px column, map starts at 117: 40% of the 547px below -> 218
  assert.equal(V.phoneStageHeight(664, 117, 480), 218);
  assert.equal(V.phoneStageHeight(400, 117, 480), 140); // never below the minimum
  assert.equal(V.phoneStageHeight(2000, 100, 480), 480); // never taller than the town
  assert.equal(V.stageHeight, undefined); // the old "fill down to a fixed chat panel" sizing is gone
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
  assert.match(line.text, /らぼみは今つながっていないので届かない/);
  assert.doesNotMatch(line.text, /誰にも/); // the town still records it (and extgate reports every knock)
  const ok = Chat.entry({ type: "knock", by: "labomi", house: "nostarou-house" }, { houses: ts });
  assert.doesNotMatch(ok.text, /届かない/);
  assert.match(Chat.resultText({ cmd: "move", ok: false, error: "forbidden", role: "owner" }), /招待/);
});

test("the knock button has a house picker, not a fixed nostarou-house", () => {
  const html = fs.readFileSync(path.join(__dirname, "../web/nostr.html"), "utf8");
  const js = fs.readFileSync(path.join(__dirname, "../web-src/nostr.ts"), "utf8");
  assert.match(html, /<select[^>]*id="knockTo"/);
  assert.doesNotMatch(js, /type: "knock", room: "nostarou-house"/);
  // phone text sizes: chat 17px+ (bigger than the 16px controls), status / buttons 15px
  const css = html.slice(html.indexOf("<style>"), html.indexOf("</style>"));
  const logPx = Number((css.match(/body\.narrow #log \{[^}]*font-size:(\d+)px/) || [])[1]);
  assert.ok(logPx >= 20, "phone chat log font-size >= 20px, got " + logPx);
  assert.match(html, /id="ver"[^>]*>v:dev</); // Pages build stamps the commit here
  const widePx = Number((css.match(/\n  #log \{[^}]*font:(\d+)px/) || [])[1]);
  assert.ok(widePx >= 16, "wide chat log font >= 16px, got " + widePx);
  assert.match(css, /body\.narrow #status \{[^}]*font-size:15px/);
  assert.match(css, /body\.narrow button \{[^}]*font-size:15px/);
});
