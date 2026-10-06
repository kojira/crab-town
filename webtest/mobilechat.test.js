// node --test webtest/*.test.js : the phone chat panel (iPhone Safari, 390px).
// The page is one column the height of the visual viewport: header, map, then
// the chat panel down to the bottom edge (no band between the map and the
// chat). The log takes the rest of the panel, the input stays at the bottom,
// and the soft keyboard shrinks the column instead of covering the input.
const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const Chat = require("../web/chatlog.js");

const html = fs.readFileSync(path.join(__dirname, "../web/nostr.html"), "utf8");
const css = html.slice(html.indexOf("<style>") + 7, html.indexOf("</style>")).replace(/\/\*[\s\S]*?\*\//g, "");
// the declarations of the first rule whose selector list is exactly sel
function rule(sel) {
  for (const m of css.matchAll(/([^{}]+)\{([^}]*)\}/g)) if (m[1].trim() === sel) return m[2];
  assert.fail("no rule " + sel);
}
const px = (body, prop) => { const m = body.match(new RegExp("(?:^|;)\\s*" + prop + ":\\s*([\\d.]+)px")); return m ? Number(m[1]) : null; };

test("phone page is a column pinned to the visual viewport", () => {
  const b = rule("body.narrow");
  assert.match(b, /position:fixed/);
  assert.match(b, /height:var\(--app-h/);
  assert.match(b, /top:var\(--vv-top/);
  assert.match(b, /display:flex/);
  assert.match(b, /flex-direction:column/);
  // the old layout reserved the chat height as page padding: the source of the empty band
  assert.doesNotMatch(css, /padding-bottom:var\(--chat-h/);
});

test("the chat panel runs from under the map to the bottom; the log takes the rest", () => {
  const chat = rule("body.narrow #chat");
  assert.match(chat, /flex:1 1 0/);
  assert.match(chat, /min-height:0/);
  assert.doesNotMatch(chat, /position:fixed/); // not laid over the page any more
  assert.match(rule("body.narrow #logwrap"), /flex:1 1 0/);
  const log = rule("body.narrow #log");
  assert.match(log, /height:100%/);
  assert.doesNotMatch(log, /height:min\(/); // no fixed 20vh / 180px log
  assert.ok(px(log, "font-size") >= 20, "chat text >= 20px");
  // nothing sits between the map and the chat: the status line lies over the map
  const mapwrap = html.slice(html.indexOf('<div id="mapwrap"'), html.indexOf('<div id="chat"'));
  assert.ok(mapwrap.includes('id="status"'), "#status is inside #mapwrap");
  assert.match(rule("body.narrow #status"), /position:absolute/);
  assert.match(rule("body.narrow #relays"), /display:none/);
});

test("phone input >= 16px (no focus zoom) and a send button big enough to tap", () => {
  assert.ok(px(rule("body.narrow #talkText"), "font-size") >= 16);
  assert.ok(px(rule("body.narrow #talkText"), "min-height") >= 44);
  const send = rule("body.narrow #talk");
  assert.ok(px(send, "min-height") >= 44, "send button >= 44px tall");
  assert.ok(px(send, "min-width") >= 44, "send button >= 44px wide");
  assert.ok(px(send, "font-size") >= 16);
  assert.match(rule("body.narrow #talkForm"), /margin:auto 0 0/); // pinned to the bottom of the panel
});

test("keyboard up: the rows above the log fold away, the map keeps its height, the log and input stay", () => {
  const fold = rule("body.narrow.kbup h1, body.narrow.kbup #bar, body.narrow.kbup #jumps, body.narrow.kbup #chatHead");
  assert.match(fold, /display:none/);
  // focus / typing / blur must not resize the map (iPhone: it shrank on focus and came back on blur)
  assert.doesNotMatch(css, /kbup[^{]*#(mapwrap|stage)\b[^{]*\{[^}]*height/);
  assert.doesNotMatch(css, /kbup[^{]*#(log|talkForm|talkText|talk)\b[^{]*\{[^}]*display:none/);
});

test("appViewport: the visible part of the page, from visualViewport", () => {
  assert.deepEqual(Chat.appViewport(664, null), { height: 664, top: 0 });               // no visualViewport
  assert.deepEqual(Chat.appViewport(664, { height: 664, offsetTop: 0 }), { height: 664, top: 0 });
  assert.deepEqual(Chat.appViewport(664, { height: 328, offsetTop: 0 }), { height: 328, top: 0 }); // iOS keyboard
  assert.deepEqual(Chat.appViewport(664, { height: 328.4, offsetTop: 40 }), { height: 328, top: 40 }); // scrolled under it
  assert.deepEqual(Chat.appViewport(664, { height: 700, offsetTop: -3 }), { height: 664, top: 0 }); // never taller than the window
  assert.deepEqual(Chat.appViewport(664, { height: 0 }), { height: 664, top: 0 });
});

test("viewportMoved: only a real change of the visible column re-lays out the page", () => {
  const v = { height: 664, top: 0 };
  assert.equal(Chat.viewportMoved(v, { height: 664, top: 0 }), false);
  assert.equal(Chat.viewportMoved(v, { height: 665, top: 1 }), false); // keyboard animation jitter
  assert.equal(Chat.viewportMoved(v, { height: 328, top: 0 }), true);  // keyboard up
  assert.equal(Chat.viewportMoved(v, { height: 664, top: 40 }), true);
  assert.equal(Chat.viewportMoved({ height: -1, top: -1 }, v), true);  // first call
});
