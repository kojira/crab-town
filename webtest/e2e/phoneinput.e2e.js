// pnpm test:e2e : the phone talk input in Playwright WebKit (iPhone 13, 390x844).
// iOS Safari's soft keyboard and pinch zoom only move window.visualViewport, so
// a stand-in visualViewport (height / offsetTop / scale + resize/scroll events)
// plays the keyboard. Checks:
//  - every form control's computed font-size is >= 16px (no iOS focus zoom)
//  - focus -> typing one character at a time -> keyboard up -> map tap (blur):
//    the map keeps its height at every step
//  - a burst of visualViewport resize/scroll events re-lays out the page a
//    bounded number of times (no flicker loop), and a pinch-zoomed viewport
//    (scale != 1) does not resize the page column at all
// WEB_DIR=<dir> runs it against another copy of web/ (e.g. the code before a fix);
// SHOTS=<dir> saves a screenshot per step.
const test = require("node:test");
const assert = require("node:assert/strict");
const http = require("node:http");
const fs = require("node:fs");
const path = require("node:path");

let pw = null;
try { pw = require("playwright"); } catch { /* not installed: skipped below */ }
const WEB = path.resolve(process.env.WEB_DIR || path.join(__dirname, "../../web"));
const SHOTS = process.env.SHOTS || "";

function serve() {
  const srv = http.createServer((q, r) => {
    const u = decodeURIComponent(q.url.split("?")[0]);
    const f = path.join(WEB, u === "/" ? "nostr.html" : u);
    if (!f.startsWith(WEB)) { r.writeHead(403); return r.end(); }
    fs.readFile(f, (e, d) => {
      if (e) { r.writeHead(404); return r.end(); }
      r.writeHead(200, { "content-type": f.endsWith(".html") ? "text/html; charset=utf-8" : "text/javascript" });
      r.end(d);
    });
  });
  return new Promise(ok => srv.listen(0, "127.0.0.1", () => ok(srv)));
}

// a visualViewport the test drives (height / offsetTop / scale), like iOS's
function fakeViewport() {
  const f = new EventTarget();
  Object.assign(f, { width: innerWidth, height: innerHeight, offsetTop: 0, offsetLeft: 0, pageTop: 0, pageLeft: 0, scale: 1 });
  Object.defineProperty(window, "visualViewport", { configurable: true, get: () => f });
  window.__vv = f;
  window.__vvSet = (o) => { Object.assign(f, o); };
  window.__fire = (type) => f.dispatchEvent(new Event(type));
}

async function open(browser, height = 844) {
  const ctx = await browser.newContext({ ...pw.devices["iPhone 13"], viewport: { width: 390, height } });
  const page = await ctx.newPage();
  await page.addInitScript(fakeViewport);
  await page.routeWebSocket(/.*/, () => {}); // no relays: the page shows an empty town
  const srv = await serve();
  await page.goto(`http://127.0.0.1:${srv.address().port}/nostr.html`);
  await page.waitForFunction(() => document.body.classList.contains("narrow"));
  await page.evaluate(() => { document.getElementById("talkText").disabled = false; });
  await page.waitForTimeout(300);
  return { page, close: async () => { await ctx.close(); srv.close(); } };
}

const mapH = (page) => page.evaluate(() => ({
  mapwrap: Math.round(document.getElementById("mapwrap").getBoundingClientRect().height),
  stage: Math.round(document.getElementById("stage").getBoundingClientRect().height),
}));
const shot = async (page, name) => { if (SHOTS) { fs.mkdirSync(SHOTS, { recursive: true }); await page.screenshot({ path: path.join(SHOTS, name + ".png") }); } };
const frames = (page, n = 4) => page.evaluate((n) => new Promise(ok => { const f = (i) => i ? requestAnimationFrame(() => f(i - 1)) : ok(); f(n); }), n);

const skip = !pw ? "playwright is not installed" : false;
let browser;
test.before(async () => { if (pw) browser = await pw.webkit.launch(); });
test.after(async () => { if (browser) await browser.close(); });

for (const h of [844, 500]) {
  test(`form controls are >= 16px computed (iPhone 13, 390x${h})`, { skip }, async () => {
    const { page, close } = await open(browser, h);
    try {
      const sizes = async () => page.evaluate(() => [...document.querySelectorAll("input, select, textarea")]
        .map(e => ({ id: e.id || e.type, px: parseFloat(getComputedStyle(e).fontSize), tf: getComputedStyle(e).transform })));
      const check = (list, when) => {
        assert.ok(list.some(x => x.id === "talkText"), "talkText is there");
        for (const x of list) {
          assert.ok(x.px >= 16, `${when}: ${x.id} is ${x.px}px`);
          assert.equal(x.tf, "none", `${when}: ${x.id} is transformed`);
        }
      };
      check(await sizes(), "idle");
      await page.tap("#talkText");
      await page.waitForTimeout(400);
      check(await sizes(), "focused");
      // nothing above the input scales it down either
      const chain = await page.evaluate(() => { const out = []; for (let e = document.getElementById("talkText"); e; e = e.parentElement) { const s = getComputedStyle(e); if (s.transform !== "none" || (s.zoom && s.zoom !== "1" && s.zoom !== "normal")) out.push(e.tagName + "#" + e.id); } return out; });
      assert.deepEqual(chain, []);
    } finally { await close(); }
  });

  test(`focus / typing / keyboard / blur keep the map height (390x${h})`, { skip }, async () => {
    const { page, close } = await open(browser, h);
    try {
      const tag = h === 844 ? "" : "-" + h;
      const base = await mapH(page);
      await shot(page, "0-idle" + tag);
      await page.tap("#talkText");
      await page.waitForTimeout(400);
      await shot(page, "1-focus" + tag);
      assert.deepEqual(await mapH(page), base, "after focus");
      for (const ch of "こんにちはabc") {
        await page.keyboard.type(ch);
        await frames(page, 2);
        assert.deepEqual(await mapH(page), base, "after typing " + ch);
      }
      // the soft keyboard: the visual viewport shrinks by 45% (innerHeight stays)
      await page.evaluate(() => { window.__vvSet({ height: innerHeight - Math.round(innerHeight * 0.45) }); window.__fire("resize"); });
      await page.waitForTimeout(400);
      await shot(page, "2-keyboard" + tag);
      assert.deepEqual(await mapH(page), base, "keyboard up");
      // tap the map: the input blurs, the keyboard goes away
      await page.tap("#stage", { position: { x: 40, y: 30 } });
      await page.evaluate(() => { window.__vvSet({ height: innerHeight }); window.__fire("resize"); });
      await page.waitForTimeout(600);
      assert.notEqual(await page.evaluate(() => document.activeElement && document.activeElement.id), "talkText", "blurred");
      await shot(page, "3-blur" + tag);
      assert.deepEqual(await mapH(page), base, "after blur");
    } finally { await close(); }
  });
}

test("a burst of visualViewport events re-lays out a bounded number of times", { skip }, async () => {
  const { page, close } = await open(browser);
  try {
    await page.tap("#talkText");
    await page.waitForTimeout(400);
    // count writes to the page column and the map
    await page.evaluate(() => {
      window.__writes = 0;
      const mo = new MutationObserver(ms => { window.__writes += ms.length; });
      mo.observe(document.body, { attributes: true, attributeFilter: ["style", "class"] });
      mo.observe(document.getElementById("stage"), { attributes: true, attributeFilter: ["style"] });
      mo.observe(document.getElementById("c"), { attributes: true, attributeFilter: ["style"] });
    });
    // keyboard animating in: 120 events, heights jittering by fractions of a px
    await page.evaluate(() => {
      const h0 = innerHeight;
      for (let i = 0; i < 120; i++) {
        window.__vvSet({ height: h0 - 380 + (i % 2 ? 0.6 : 0.2), offsetTop: i % 3 ? 0 : 0.4 });
        window.__fire(i % 2 ? "resize" : "scroll");
      }
    });
    await frames(page, 6);
    const burst = await page.evaluate(() => window.__writes);
    assert.ok(burst <= 6, `120 events -> ${burst} layout writes`);
    // jitter alone (no real change) writes nothing
    await page.evaluate(() => { window.__writes = 0; for (let i = 0; i < 60; i++) { window.__vvSet({ height: window.__vv.height + (i % 2 ? 0.5 : -0.5) }); window.__fire("resize"); } });
    await frames(page, 6);
    assert.equal(await page.evaluate(() => window.__writes), 0, "sub-2px jitter re-lays out nothing");
  } finally { await close(); }
});

test("a pinch-zoomed viewport (scale != 1) does not resize the page column", { skip }, async () => {
  const { page, close } = await open(browser);
  try {
    const col = () => page.evaluate(() => ({ h: Math.round(document.body.getBoundingClientRect().height), app: document.body.style.getPropertyValue("--app-h"), top: document.body.style.getPropertyValue("--vv-top") }));
    const before = await col(), map = await mapH(page);
    await page.evaluate(() => { window.__vvSet({ scale: 2, height: innerHeight / 2, width: innerWidth / 2, offsetTop: 120 }); window.__fire("resize"); window.__fire("scroll"); });
    await frames(page, 6);
    assert.deepEqual(await col(), before);
    assert.deepEqual(await mapH(page), map);
  } finally { await close(); }
});
