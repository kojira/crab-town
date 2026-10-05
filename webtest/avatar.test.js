// node --test webtest/*.test.js : the logged-in owner's avatar (kind:0 picture, round, initial fallback).
const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const A = require("../web/avatar.js");
const PK = "ab".repeat(32);

test("kind:0: name and http(s) picture; anything else is no picture", () => {
  assert.deepEqual(A.parseProfile({ content: JSON.stringify({ name: "kojira", picture: "https://x/a.png" }) }), { name: "kojira", picture: "https://x/a.png" });
  assert.equal(A.parseProfile({ content: JSON.stringify({ display_name: "こじら", name: "k" }) }).name, "こじら");
  assert.equal(A.parseProfile({ content: JSON.stringify({ picture: "javascript:alert(1)" }) }).picture, "");
  assert.equal(A.parseProfile({ content: "not json" }).picture, "");
  assert.equal(A.parseProfile(null).name, "");
});

test("newest kind:0 of that pubkey wins", () => {
  const evs = [
    { kind: 0, pubkey: PK, created_at: 1, content: "old" },
    { kind: 0, pubkey: PK, created_at: 3, content: "new" },
    { kind: 0, pubkey: "cd".repeat(32), created_at: 9, content: "someone else" },
    { kind: 1, pubkey: PK, created_at: 9, content: "a note" },
  ];
  assert.equal(A.pickLatest(evs, PK).content, "new");
  assert.equal(A.pickLatest([], PK), null);
});

test("no picture or a broken one: the initial is drawn", () => {
  assert.equal(A.mode(undefined), "initial");
  assert.equal(A.mode({ status: "loading" }), "initial");
  assert.equal(A.mode({ status: "initial", picture: "https://x/broken.png" }), "initial");
  assert.equal(A.mode({ status: "image", img: {} }), "image");
  assert.equal(A.initial("kojira", PK), "K");
  assert.equal(A.initial("こじら", PK), "こ");
  assert.equal(A.initial("", PK), "A");
  assert.equal(A.hue(PK), A.hue(PK));
});

// a fake 2D context recording calls
function fakeCtx() {
  const calls = [];
  const rec = (name) => (...args) => calls.push([name, ...args]);
  return { calls, save: rec("save"), restore: rec("restore"), beginPath: rec("beginPath"), arc: rec("arc"), closePath: rec("closePath"),
    clip: rec("clip"), drawImage: rec("drawImage"), fill: rec("fill"), fillText: rec("fillText"), stroke: rec("stroke") };
}

test("the picture is clipped to a tile-sized circle; without it the initial", () => {
  const img = { w: 400 };
  A.profiles[PK] = { name: "kojira", status: "image", img };
  let c = fakeCtx();
  A.draw(c, { pubkey: PK, name: "オーナー" }, 64, 32, 32);
  const names = c.calls.map(x => x[0]);
  assert.ok(names.indexOf("clip") >= 0 && names.indexOf("clip") < names.indexOf("drawImage"));
  const di = c.calls.find(x => x[0] === "drawImage");
  assert.equal(di[1], img);
  assert.deepEqual(di.slice(2), [65, 33, 30, 30]); // inside the 32px tile
  assert.deepEqual(c.calls.find(x => x[0] === "arc").slice(1, 4), [80, 48, 15]);
  assert.equal(A.label({ pubkey: PK, name: "オーナー" }), "kojira");

  A.profiles[PK] = { name: "kojira", status: "initial", picture: "https://x/broken.png" };
  c = fakeCtx();
  A.draw(c, { pubkey: PK, name: "オーナー" }, 64, 32, 32);
  assert.equal(c.calls.find(x => x[0] === "drawImage"), undefined);
  assert.equal(c.calls.find(x => x[0] === "fillText")[1], "K");
  delete A.profiles[PK];
});

test("render.js draws a pubkey actor as the avatar, residents keep their sprites", () => {
  const src = fs.readFileSync(path.join(__dirname, "../web/render.js"), "utf8");
  assert.match(src, /a\.pubkey && typeof CrabAvatar/);
  assert.match(src, /CrabAvatar\.draw\(ctx, a,/);
  for (const f of ["nostr.html", "index.html"]) {
    const html = fs.readFileSync(path.join(__dirname, "../web", f), "utf8");
    assert.ok(html.indexOf("avatar.js") > 0 && html.indexOf("avatar.js") < html.indexOf("render.js"), f);
  }
});

test("the Pages viewer no longer drives nostarou: the owner's actor is its own avatar", () => {
  const src = fs.readFileSync(path.join(__dirname, "../web/nostr.js"), "utf8");
  assert.doesNotMatch(src, /ownerActor/);
  assert.match(src, /myActor = ownAvatarId\(me\)/);
  assert.match(src, /CrabAvatar\.load\(a\.pubkey, RELAYS, verifyEvent\)/);
  const cfg = fs.readFileSync(path.join(__dirname, "../web/config.js"), "utf8");
  assert.doesNotMatch(cfg, /ownerActor/);
  assert.match(cfg, /r\.kojira\.io.*n\.kojira\.io.*x\.kojira\.io/);
});
