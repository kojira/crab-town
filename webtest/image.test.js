// node --test webtest/*.test.js : chat images (issue #20, owner only).
const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const crypto = require("node:crypto");
const U = require("../web/upload.js");
const Chat = require("../web/chatlog.js");
const read = (f) => fs.readFileSync(path.join(__dirname, "../web", f), "utf8");
const src = (f) => fs.readFileSync(path.join(__dirname, "../web-src", f), "utf8");

test("only https image URLs are drawn", () => {
  assert.equal(U.safeImageUrl("https://blossom.example/ab.png"), "https://blossom.example/ab.png");
  for (const bad of ["http://x.example/a.png", "javascript:alert(1)", "data:image/png;base64,AA", "//x.example/a.png",
    "https://u:p@x.example/a.png", "https://x.example/a b.png", 'https://x.example/"onerror="x', "https://x.example/<b>",
    "https://x.example/" + "a".repeat(1100), "", null, undefined, 42, "/relative.png"]) {
    assert.equal(U.safeImageUrl(bad), null, String(bad));
  }
});

test("files: images only, 5MB at most", () => {
  assert.equal(U.checkFile({ type: "image/png", size: 10 }), "");
  assert.match(U.checkFile({ type: "image/svg+xml", size: 10 }), /画像/); // svg can carry script
  assert.match(U.checkFile({ type: "text/html", size: 10 }), /画像/);
  assert.match(U.checkFile({ type: "image/jpeg", size: U.MAX_BYTES + 1 }), /5MB/);
  assert.match(U.checkFile({ type: "image/jpeg", size: 0 }), /空/);
});

test("Blossom upload auth is a kind 24242 with t=upload, x=sha256 and an expiration", () => {
  const t = U.authTemplate("ab".repeat(32), 1000);
  assert.equal(t.kind, 24242);
  assert.deepEqual(t.tags, [["t", "upload"], ["x", "ab".repeat(32)], ["expiration", "1300"]]);
  const h = U.authHeader({ kind: 24242, content: "日本語" });
  assert.match(h, /^Nostr /);
  assert.deepEqual(JSON.parse(Buffer.from(h.slice(6), "base64").toString("utf8")), { kind: 24242, content: "日本語" });
});

// The upload against a fake Blossom server (no real upload happens in tests).
test("upload: PUT /upload with the signed auth, returns the descriptor's https url", async () => {
  const body = new Uint8Array([137, 80, 78, 71, 1, 2, 3]);
  const sha = crypto.createHash("sha256").update(body).digest("hex");
  const file = new Blob([body], { type: "image/png" });
  let seen;
  const deps = {
    digest: async (d) => crypto.webcrypto.subtle.digest("SHA-256", d),
    sign: async (t) => ({ ...t, id: "i", pubkey: "p", sig: "s" }),
    now: () => 1_700_000_000_000,
    fetch: async (url, init) => { seen = { url, init }; return new Response(JSON.stringify({ url: `https://b.example/${sha}.png`, sha256: sha, size: 7 }), { status: 200 }); },
  };
  const url = await U.upload("https://b.example/", file, deps);
  assert.equal(url, `https://b.example/${sha}.png`);
  assert.equal(seen.url, "https://b.example/upload");
  assert.equal(seen.init.method, "PUT");
  assert.equal(seen.init.headers["X-SHA-256"], sha);
  const auth = JSON.parse(Buffer.from(seen.init.headers.Authorization.slice(6), "base64").toString());
  assert.deepEqual(auth.tags[1], ["x", sha]);
  assert.equal(auth.created_at, 1_700_000_000);
  // a refusal, an http url, a different hash: errors in words, no url
  await assert.rejects(U.upload("https://b.example", file, { ...deps, fetch: async () => new Response("no", { status: 401, headers: { "X-Reason": "auth" } }) }), /401 auth/);
  await assert.rejects(U.upload("https://b.example", file, { ...deps, fetch: async () => new Response(JSON.stringify({ url: "http://b.example/x.png" })) }), /https/);
  await assert.rejects(U.upload("https://b.example", file, { ...deps, fetch: async () => new Response(JSON.stringify({ url: "https://b.example/x.png", sha256: "00" })) }), /https/);
  await assert.rejects(U.upload("http://b.example", file, deps), /https/);
  await assert.rejects(U.upload("https://b.example", new Blob(["<svg/>"], { type: "image/svg+xml" }), deps), /画像/);
});

test("a chat line carries an image only from an owner talk", () => {
  const img = "https://b.example/a.png";
  assert.equal(Chat.entry({ type: "talk", by: "nostr:o", role: "owner", to: "nostarou", message: "", image: img }).image, img);
  assert.equal(Chat.entry({ type: "talk", by: "nostr:g", role: "guest", to: "nostarou", message: "x", image: img }).image, undefined);
});

test("images are drawn with DOM properties, never innerHTML", () => {
  const code = (f) => src(f).replace(/\/\/.*$/gm, ""); // comments may say "no innerHTML"
  for (const f of ["upload.ts", "chatlog.ts", "nostr.ts"]) assert.doesNotMatch(code(f), /innerHTML|outerHTML|insertAdjacentHTML|document\.write/, f);
  assert.match(src("upload.ts"), /img\.src = safe/);
  assert.match(src("chatlog.ts"), /CrabUpload\.imageNode\(document, e\.image\)/);
});

test("the image button is hidden until the town answers this pubkey as owner", () => {
  const html = read("nostr.html");
  assert.match(html, /<label id="imgBtn"[^>]*hidden>/);
  assert.match(html, /<input type="file" id="imgPick" accept="image\/png,image\/jpeg,image\/gif,image\/webp" disabled>/);
  assert.ok(html.indexOf('src="upload.js"') > 0 && html.indexOf('src="upload.js"') < html.indexOf('src="chatlog.js"'));
  const s = src("nostr.ts");
  assert.equal(s.match(/showImageButton\(\)/g).length, 2); // defined once, called once
  assert.match(s, /ev\.role === "owner" && !myActor\) \{[^}]*showImageButton\(\)/);
  // the thumbnail is capped
  assert.match(html, /#log a\.img img \{[^}]*max-width:min\(320px, 100%\); max-height:240px;/);
  // iOS Safari: the file input is not display:none (it would not open the picker)
  assert.match(html, /\.pickbtn input \{ position:absolute; inset:0;[^}]*opacity:0;/);
});
