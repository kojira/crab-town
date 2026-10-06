// web/*.js is generated from web-src/*.ts (scripts/build-web.mjs) and the
// pages load nothing from outside: nostr-tools is bundled into web/nostrtools.js.
const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");

const web = path.join(__dirname, "../web");
const read = (f) => fs.readFileSync(path.join(web, f), "utf8");

test("no script, import or stylesheet comes from another origin", () => {
  for (const f of fs.readdirSync(web).filter((f) => /\.(html|js)$/.test(f))) {
    const s = read(f);
    assert.doesNotMatch(s, /esm\.sh|unpkg\.com|jsdelivr|cdnjs/, f);
    assert.doesNotMatch(s, /<script[^>]*src="(https?:)?\/\//, f);
    assert.doesNotMatch(s, /<link[^>]*href="(https?:)?\/\//, f);
    assert.doesNotMatch(s, /\bimport\s*\(\s*["']https?:/, f);
    assert.doesNotMatch(s, /^\s*import\b[^;]*from\s*["']https?:/m, f);
  }
});

test("nostr.html loads the local nostr-tools bundle before nostr.js", () => {
  const html = read("nostr.html");
  const at = html.indexOf('<script src="nostrtools.js">');
  assert.ok(at > 0 && at < html.indexOf('src="nostr.js"'));
  assert.match(read("nostrtools.js"), /var NostrTools\s*=/);
});

test("every web-src script has its generated web/*.js", () => {
  const src = fs.readdirSync(path.join(__dirname, "../web-src")).filter((f) => f.endsWith(".ts") && !f.endsWith(".d.ts"));
  for (const f of src) assert.match(read(f.replace(/\.ts$/, ".js")), /^\/\/ generated from web-src\//, f);
});
