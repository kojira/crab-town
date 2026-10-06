// node --test webtest-next/ : NIP-98 signing of the town-next viewer's calls
// (web-next-src/api.ts), checked the way internal/next/auth.go checks them.
const test = require("node:test");
const assert = require("node:assert/strict");
const crypto = require("node:crypto");
const { generateSecretKey, getPublicKey, finalizeEvent, verifyEvent } = require("nostr-tools/pure");
const A = require("./load")("api");

const sk = generateSecretKey();
const signer = { getPublicKey: async () => getPublicKey(sk), signEvent: async (t) => finalizeEvent(t, sk) };
const decode = (h) => { assert.match(h, /^Nostr [A-Za-z0-9+/=]+$/); return JSON.parse(Buffer.from(h.slice(6), "base64").toString("utf8")); };
const tag = (ev, k) => (ev.tags.find((t) => t[0] === k) || [])[1];

function fakeFetch(status, json) {
  const calls = [];
  const f = async (url, init) => { calls.push({ url, init }); return { ok: status < 300, status, json: async () => json }; };
  f.calls = calls;
  return f;
}

for (const [op, body] of [["/move", { x: 3, y: 4 }], ["/interact", { furniture: "bed" }], ["/say", { text: "こんにちは ⚡" }], ["/plots/apply", { house: "labomi-house" }]]) {
  test(`POST ${op}: kind 27235, u = base + path, method POST, payload = sha256 of the exact body sent`, async () => {
    const f = fakeFetch(200, { ok: true, you: getPublicKey(sk) });
    const r = await A.call(signer, "http://127.0.0.1:8788/", { path: op, body }, f);
    assert.equal(r.ok, true);
    assert.equal(f.calls[0].url, "http://127.0.0.1:8788" + op);
    const sent = f.calls[0].init.body;
    assert.deepEqual(JSON.parse(sent), body);
    const ev = decode(f.calls[0].init.headers.Authorization);
    assert.equal(ev.kind, 27235);
    assert.ok(verifyEvent(ev));
    assert.equal(ev.pubkey, getPublicKey(sk));
    assert.equal(tag(ev, "u"), "http://127.0.0.1:8788" + op);
    assert.equal(tag(ev, "method").toUpperCase(), "POST");
    assert.equal(tag(ev, "payload"), crypto.createHash("sha256").update(Buffer.from(sent, "utf8")).digest("hex"));
    assert.ok(Math.abs(ev.created_at - Date.now() / 1000) < 5);
  });
}

test("POST /join has no body and no payload tag", async () => {
  const f = fakeFetch(200, { ok: true });
  await A.call(signer, "http://h:1", { path: "/join" }, f);
  assert.equal(f.calls[0].init.body, undefined);
  assert.equal(tag(decode(f.calls[0].init.headers.Authorization), "payload"), undefined);
});

test("an error keeps the status and the server's message", async () => {
  const r = await A.call(signer, "http://h:1", { path: "/move", body: { x: 1, y: 1 } }, fakeFetch(409, { ok: false, error: "tile is taken" }));
  assert.deepEqual([r.ok, r.status, r.error], [false, 409, "tile is taken"]);
});

test("/world: anonymous without a signer, ?auth= with a GET /world event with one", async () => {
  assert.equal(await A.worldURL("http://127.0.0.1:8788"), "ws://127.0.0.1:8788/world");
  const u = new URL(await A.worldURL("https://town.example/", signer));
  assert.equal(u.origin + u.pathname, "wss://town.example/world");
  const ev = decode(u.searchParams.get("auth"));
  assert.equal(tag(ev, "u"), "https://town.example/world");
  assert.equal(tag(ev, "method").toUpperCase(), "GET");
});
