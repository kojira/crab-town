// pnpm test:e2e:next : the town-next viewer against a real town-next server.
// The server is built from cmd/town-next and run on a free 127.0.0.1 port with
// a throwaway copy of town-next.example.json under .work/ (never the real data,
// never 8788). The page signs with a window.nostr whose key lives in this test
// (exposeFunction), so every click is a real NIP-98 round trip:
//   join -> tap a tile (/move) -> [行動] use a bench (/interact) -> say (/say)
//   -> tap a vacant plot (/plots/apply)
// and the layout rules of docs/town-next-ui.md 1-2 are checked on an iPhone 13
// (390x844, WebKit) and a PC (1280x800). SHOTS=<dir> saves screenshots.
const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const net = require("node:net");
const path = require("node:path");
const { spawn, execFileSync } = require("node:child_process");
const { generateSecretKey, getPublicKey, finalizeEvent } = require("nostr-tools/pure");

let pw = null;
try { pw = require("playwright"); } catch { /* not installed: skipped below */ }
const ROOT = path.join(__dirname, "../..");
const WORK = path.join(ROOT, ".work", "e2e-next");
const SHOTS = process.env.SHOTS || "";
const HOLDER = generateSecretKey(); // the PC test's key: holder of nostarou-house in the throwaway data

const freePort = () => new Promise((ok) => { const s = net.createServer(); s.listen(0, "127.0.0.1", () => { const p = s.address().port; s.close(() => ok(p)); }); });

async function startTown() {
  fs.mkdirSync(WORK, { recursive: true });
  const bin = path.join(WORK, "town-next");
  execFileSync("go", ["build", "-o", bin, "./cmd/town-next"], { cwd: ROOT, stdio: "inherit" });
  const data = path.join(WORK, `town-${process.pid}.json`);
  const d = JSON.parse(fs.readFileSync(path.join(ROOT, "town-next.example.json"), "utf8"));
  const hpk = getPublicKey(HOLDER);
  d.rooms[0].houses.find((h) => h.id === "nostarou-house").owner = hpk;
  d.names = [{ pubkey: hpk, name: "家主さん", room: "town", pos: { x: 48, y: 3 } }];
  fs.writeFileSync(data, JSON.stringify(d));
  const port = await freePort();
  const base = `http://127.0.0.1:${port}`;
  const p = spawn(bin, [], { env: { ...process.env, TOWN_NEXT_DATA: data, TOWN_NEXT_ADDR: `127.0.0.1:${port}`, TOWN_NEXT_BASE_URL: base, TOWN_NEXT_TICK: "50ms" }, stdio: "inherit" });
  for (let i = 0; i < 100; i++) {
    try { if ((await fetch(base + "/")).ok) break; } catch { /* not yet */ }
    await new Promise((r) => setTimeout(r, 100));
  }
  return { base, data, stop: () => { p.kill(); fs.rmSync(data, { force: true }); } };
}

// a window.nostr backed by a key held in this test process
async function withSigner(page, sk) {
  await page.exposeFunction("__pk", () => getPublicKey(sk));
  await page.exposeFunction("__sign", (t) => finalizeEvent(t, sk));
  await page.addInitScript(() => { window.nostr = { getPublicKey: () => window.__pk(), signEvent: (t) => window.__sign(t) }; });
}

const skip = !pw ? "playwright is not installed" : false;
let town, webkit, chromium;
test.before(async () => { if (!pw) return; town = await startTown(); webkit = await pw.webkit.launch(); chromium = await pw.chromium.launch(); });
test.after(async () => { await webkit?.close(); await chromium?.close(); town?.stop(); });

const shot = async (page, name) => { if (SHOTS) { fs.mkdirSync(SHOTS, { recursive: true }); await page.screenshot({ path: path.join(SHOTS, name + ".png") }); } };
const rect = (page, sel) => page.$eval(sel, (e) => { const r = e.getBoundingClientRect(); return { x: r.x, y: r.y, w: r.width, h: r.height }; });
const me = (page, pk) => page.evaluate((pk) => window.__townNext?.state?.actors[pk], pk);

async function login(page, pk) {
  await page.click("#login");
  await page.click("#actbtn");
  await page.getByRole("button", { name: "町に入る" }).click();
  await page.waitForFunction((pk) => !!window.__townNext?.state?.actors[pk], pk);
}

// tap the tile (x, y) of the town on the map canvas
async function tapTile(page, x, y) {
  const p = await page.evaluate(([x, y]) => window.__townNext.tileToScreen(x, y), [x, y]);
  const r = await rect(page, "#map");
  await page.mouse.click(r.x + p.x, r.y + p.y);
}

test("iPhone 13 (390x844): layout of 1.1, real join / move / interact / say / apply, keyboard of 2", { skip }, async () => {
  const ctx = await webkit.newContext({ ...pw.devices["iPhone 13"], hasTouch: false });
  const page = await ctx.newPage();
  const sk = generateSecretKey(), pk = getPublicKey(sk);
  await withSigner(page, sk);
  // a stand-in visualViewport the test drives, like iOS's (the soft keyboard only shrinks it)
  await page.addInitScript(() => {
    const f = new EventTarget(); Object.assign(f, { width: innerWidth, height: innerHeight, offsetTop: 0, scale: 1 });
    Object.defineProperty(window, "visualViewport", { configurable: true, get: () => f });
    window.__vv = f;
  });
  try {
    await page.goto(town.base + "/");
    await page.waitForFunction(() => window.__townNext?.state?.rooms.length > 0);
    assert.equal(await page.evaluate(() => document.body.dataset.layout), "phone");
    const vh = await page.evaluate(() => innerHeight);
    const map0 = await rect(page, "#mapwrap");
    assert.ok(Math.abs(map0.h - vh * 0.58) <= 2, `map is 58svh: ${map0.h} of ${vh}`);
    assert.equal(Math.round(map0.w), 390);
    const mini = await rect(page, "#mini");
    assert.ok(mini.y < map0.y + map0.h && mini.x > map0.w / 2, "minimap overlaid at the top right of the map");
    assert.ok(Math.abs(mini.w - 390 * 0.3) <= 2, `minimap 30% wide: ${mini.w}`);
    const fonts = await page.$$eval("input, textarea, select", (l) => l.map((e) => parseFloat(getComputedStyle(e).fontSize)));
    assert.ok(fonts.length > 0 && fonts.every((f) => f >= 16), `inputs >= 16px: ${fonts}`);
    assert.doesNotMatch(await page.$eval("meta[name=viewport]", (m) => m.content), /maximum-scale|user-scalable/);
    const chat = await rect(page, "#chat");
    assert.ok(chat.h >= 4 * 16 * 1.45, `chat holds 4 lines: ${chat.h}`);
    await shot(page, "phone-390-before-login");

    await login(page, pk);
    const a0 = await me(page, pk);
    assert.deepEqual(a0.pos, { x: 24, y: 5 }, "a new key appears at the spawn");
    // tap a garden tile: /move
    await tapTile(page, 24, 8);
    await page.waitForFunction((pk) => { const a = window.__townNext?.state.actors[pk]; return a.pos.x === 24 && a.pos.y === 8; }, pk);
    // [行動] -> the postbox is decoration; walk next to nothing usable: the list offers plots
    await page.click("#actbtn");
    assert.ok(await page.getByRole("button", { name: "区画 labomi-house を申請" }).isVisible());
    await page.click("#sheetclose");
    // say
    await page.fill("#say", "こんにちは ⚡");
    await page.click("#send");
    await page.waitForFunction(() => document.getElementById("chatlog").textContent.includes("こんにちは ⚡"));
    assert.equal(await page.inputValue("#say"), "", "the input is cleared after /say succeeded");
    // tap the vacant plot (labomi-house, inside its LDK): apply
    await tapTile(page, 20, 4);
    await page.getByRole("button", { name: "この区画を申請" }).click();
    for (let i = 0; i < 50; i++) { if (JSON.parse(fs.readFileSync(town.data, "utf8")).applications) break; await page.waitForTimeout(100); }
    assert.deepEqual(JSON.parse(fs.readFileSync(town.data, "utf8")).applications, [{ house: "labomi-house", pubkey: pk }], "/plots/apply reached the data file");
    await shot(page, "phone-390");
    assert.equal(await page.$eval("#peek", (e) => e.hidden), true, "no peek without the keyboard");
    const at0 = await page.evaluate((pk) => window.__townNext.tileToScreen(window.__townNext.state.actors[pk].pos.x, window.__townNext.state.actors[pk].pos.y), pk);

    // keyboard (2): a stand-in visualViewport shrinks by 336px -> only the input bar moves
    const bar0 = await rect(page, "#inputbar");
    await page.evaluate(() => { window.__vv.height = innerHeight - 336; window.__vv.dispatchEvent(new Event("resize")); });
    const bar1 = await rect(page, "#inputbar"), map1 = await rect(page, "#mapwrap"), chat1 = await rect(page, "#chat");
    assert.ok(Math.abs(bar0.y - bar1.y - 336) <= 1, `input bar lifted by 336: ${bar0.y} -> ${bar1.y}`);
    assert.deepEqual(map1, map0, "the map does not move or resize");
    assert.deepEqual(chat1, chat, "the chat box keeps its size");
    // the first chat line stays whole inside the chat box (not cut under the map's bottom edge)
    const log1 = await rect(page, "#chatlog"), line1 = await rect(page, "#chatlog .line");
    assert.ok(log1.y >= chat1.y - 0.5 && log1.y + log1.h <= chat1.y + chat1.h + 0.5, `the log stays inside the chat box: ${JSON.stringify(log1)} in ${JSON.stringify(chat1)}`);
    assert.ok(line1.y >= chat1.y && line1.y + line1.h <= chat1.y + chat1.h, `line 1 fully visible: ${line1.y}..${line1.y + line1.h} in ${chat1.y}..${chat1.y + chat1.h}`);
    assert.ok(line1.y >= map1.y + map1.h, `line 1 is below the map: ${line1.y} >= ${map1.y + map1.h}`);
    await shot(page, "phone-390-keyboard");
    // typing with the keyboard up: two more lines, so there are three for the peek (2 rule 6)
    for (const t of ["二行目", "三行目のちょっと長めの発言でも一行に収まって省略される ⚡⚡⚡⚡⚡⚡⚡⚡"]) {
      await page.fill("#say", t);
      await page.click("#send");
      await page.waitForFunction((t) => document.getElementById("chatlog").textContent.includes(t), t);
    }
    await shot(page, "phone-390-keyboard-3lines");
    // rule 6: the newest 3 lines, right above the lifted bar, all inside visualViewport, not under the bar
    const vv = await page.evaluate(() => ({ top: window.__vv.offsetTop, bottom: window.__vv.offsetTop + window.__vv.height }));
    assert.equal(await page.$eval("#peek", (e) => e.hidden), false, "the peek is shown while the keyboard is up");
    const want = await page.$$eval("#chatlog .line", (l) => l.slice(-3).map((e) => e.textContent));
    const got = await page.$$eval("#peek .line", (l) => l.map((e) => e.textContent));
    assert.deepEqual(got, want, "the peek holds the newest 3 lines in order");
    const peek = await rect(page, "#peek");
    const lines = await page.$$eval("#peek .line", (l) => l.map((e) => { const r = e.getBoundingClientRect(); return { y: r.y, h: r.height, x: r.x, w: r.width }; }));
    for (const l of lines) {
      assert.ok(l.h > 0 && l.y >= vv.top - 0.5 && l.y + l.h <= vv.bottom + 0.5, `peek line inside visualViewport: ${JSON.stringify(l)} in ${vv.top}..${vv.bottom}`);
      assert.ok(l.y + l.h <= bar1.y + 0.5, `peek line not under the input bar: ${l.y + l.h} <= ${bar1.y}`);
      assert.ok(l.x >= -0.5 && l.x + l.w <= 390.5, `peek line inside the width: ${JSON.stringify(l)}`);
    }
    assert.ok(Math.abs(peek.y + peek.h - bar1.y) <= 1, `the peek sits right on the bar: ${peek.y + peek.h} vs ${bar1.y}`);
    assert.deepEqual(await rect(page, "#mapwrap"), map0, "the peek does not resize or move the map");
    // rule 7: my whole drawing (name tag .. circle) is inside visualViewport and above the peek and the bar
    await page.evaluate(() => new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r))));
    const span = await page.evaluate((pk) => window.__townNext.actorSpan(pk), pk);
    const sTop = map0.y + span.top, sBot = map0.y + span.bottom;
    assert.ok(sTop >= vv.top - 0.5 && sBot <= vv.bottom + 0.5, `me inside visualViewport: ${sTop}..${sBot} in ${vv.top}..${vv.bottom}`);
    assert.ok(sTop >= map0.y - 0.5 && sBot <= map0.y + map0.h + 0.5, `me on the map: ${sTop}..${sBot}`);
    assert.ok(sBot <= peek.y + 0.5, `me not under the peek: ${sBot} <= ${peek.y}`);
    assert.ok(sBot <= bar1.y + 0.5, `me not under the input bar: ${sBot} <= ${bar1.y}`);
    // a new line while typing shows up in the peek
    await page.fill("#say", "四行目");
    await page.click("#send");
    await page.waitForFunction(() => Array.from(document.querySelectorAll("#peek .line")).pop()?.textContent.includes("四行目"));
    assert.equal(await page.$$eval("#peek .line", (l) => l.length), 3);
    // keyboard closed: back to how it was
    await page.evaluate(() => { window.__vv.height = innerHeight; window.__vv.dispatchEvent(new Event("resize")); });
    assert.equal(await page.$eval("#peek", (e) => e.hidden), true, "the peek goes away with the keyboard");
    assert.deepEqual(await rect(page, "#inputbar"), bar0, "the input bar is back");
    assert.deepEqual(await rect(page, "#mapwrap"), map0, "the map is unchanged");
    await page.evaluate(() => new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r))));
    const at1 = await page.evaluate((pk) => window.__townNext.tileToScreen(window.__townNext.state.actors[pk].pos.x, window.__townNext.state.actors[pk].pos.y), pk);
    assert.deepEqual(at1, at0, "the map's scroll is back where it was before the keyboard");
    await shot(page, "phone-390-keyboard-closed");
  } finally { await ctx.close(); }
});

test("PC (1280x800): map left 70% full height, chat on the right, furniture use and arrow keys", { skip }, async () => {
  const ctx = await chromium.newContext({ viewport: { width: 1280, height: 800 } });
  const page = await ctx.newPage();
  const sk = HOLDER, pk = getPublicKey(sk);
  await withSigner(page, sk);
  try {
    await page.goto(town.base + "/");
    await page.waitForFunction(() => window.__townNext?.state?.rooms.length > 0);
    assert.equal(await page.evaluate(() => document.body.dataset.layout), "pc");
    const map = await rect(page, "#mapwrap"), side = await rect(page, "#side");
    assert.equal(Math.round(map.w), 896);
    assert.equal(Math.round(map.h), 800);
    assert.ok(side.x >= map.w - 1 && side.h === 800, "chat column on the right");
    await login(page, pk);
    // arrow key: one /move one tile
    const a0 = await me(page, pk);
    await page.keyboard.press("ArrowDown");
    await page.waitForFunction(([pk, y]) => window.__townNext.state.actors[pk].pos.y === y, [pk, a0.pos.y + 1]);
    assert.deepEqual(a0.pos, { x: 48, y: 3 }, "a named key appears at its spot (in its own house)");
    // tap the sofa in its living room -> sheet -> 使う: /interact, walks there and uses it
    await tapTile(page, 51, 6);
    await page.getByRole("button", { name: "使う" }).click();
    await page.waitForFunction((pk) => window.__townNext.state.actors[pk].using === "sofa", pk);
    // the name tag carries rights.label from /world
    assert.equal(await page.evaluate((pk) => window.__townNext.state.rights[pk]?.label, pk), "家主");
    await shot(page, "pc-1280");
  } finally { await ctx.close(); }
});
