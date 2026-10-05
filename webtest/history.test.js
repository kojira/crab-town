// Chat history across tabs: state events are ephemeral, so the browser keeps them.
const test = require("node:test");
const assert = require("node:assert/strict");
const C = require("../web/chatlog.js");

function memStore() { const m = {}; return { getItem: k => (k in m ? m[k] : null), setItem: (k, v) => { m[k] = String(v); } }; }

test("talk / say / knock lines survive into a new tab", () => {
  const st = memStore();
  let h = [];
  h = C.keep(h, { id: "a", msg: { type: "talk", message: "やあ", at: 1 } });
  h = C.keep(h, { id: "b", msg: { type: "say", message: "おう", actor: { id: "nostarou" }, at: 2 } });
  h = C.keep(h, { id: "c", msg: { type: "knock", house: "nostarou-house", at: 3 } });
  C.saveHistory(st, h);
  const back = C.loadHistory(st);
  assert.deepEqual(back.map(x => x.id), ["a", "b", "c"]);
});

test("no duplicates, no snapshots / actors, capped to the newest", () => {
  let h = [];
  h = C.keep(h, { id: "a", msg: { type: "talk" } });
  assert.equal(C.keep(h, { id: "a", msg: { type: "talk" } }), h);
  assert.equal(C.keep(h, { id: "s", msg: { type: "snapshot" } }), h);
  assert.equal(C.keep(h, { id: "x", msg: { type: "actor" } }), h);
  for (let i = 0; i < 5; i++) h = C.keep(h, { id: "n" + i, msg: { type: "say" } }, 3);
  assert.deepEqual(h.map(x => x.id), ["n2", "n3", "n4"]);
});

test("a broken or missing store gives an empty history", () => {
  assert.deepEqual(C.loadHistory(null), []);
  assert.deepEqual(C.loadHistory({ getItem: () => "{bad" }), []);
  C.saveHistory({ setItem: () => { throw new Error("full"); } }, []);
});
