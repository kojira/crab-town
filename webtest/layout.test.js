// node --test webtest/*.test.js : the Pages viewer's chat layout (log, then the talk input under it).
const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const Chat = require("../web/chatlog.js");

const html = fs.readFileSync(path.join(__dirname, "../web/nostr.html"), "utf8");
const body = html.slice(html.indexOf("<body"));
const at = (re) => { const m = body.match(re); assert.ok(m, "missing " + re); return m.index; };
// The slice of body inside <div id="X"> ... its matching </div>.
function inner(id) {
  const start = at(new RegExp(`<div id="${id}"[^>]*>`));
  let depth = 0;
  const re = /<\/?div\b[^>]*>/g;
  re.lastIndex = start;
  for (let m; (m = re.exec(body));) {
    depth += m[0][1] === "/" ? -1 : 1;
    if (depth === 0) return body.slice(start, m.index);
  }
  assert.fail("unclosed #" + id);
}

test("talk input sits inside the chat panel, right under the log", () => {
  const chat = inner("chat");
  const log = chat.indexOf('id="log"'), form = chat.indexOf('id="talkForm"');
  assert.ok(log >= 0, "#log is in #chat");
  assert.ok(form > log, "#talkForm comes after #log inside #chat");
  assert.ok(chat.indexOf('id="talkText"') > log && chat.indexOf('id="talk"') > log);
  assert.ok(!inner("bar").includes("talkForm"), "the command bar no longer holds the talk form");
});

test("closing the chat panel hides only the log, not the input", () => {
  const css = html.slice(html.indexOf("<style>"), html.indexOf("</style>"));
  const closed = css.match(/#chat\.closed[^{]*\{[^}]*\}/g) || [];
  assert.ok(closed.some(r => r.includes("#log")), "closed hides #log");
  assert.ok(!closed.some(r => /#talk(Form|Text)?\b/.test(r)), "closed must not hide the talk input");
});

test("keyboard inset from visualViewport keeps the panel above the soft keyboard", () => {
  assert.equal(Chat.keyboardInset(844, null), 0);                          // no visualViewport
  assert.equal(Chat.keyboardInset(844, { height: 844, offsetTop: 0 }), 0); // no keyboard
  assert.equal(Chat.keyboardInset(844, { height: 508, offsetTop: 0 }), 336);
  assert.equal(Chat.keyboardInset(844, { height: 508, offsetTop: 40 }), 296); // page scrolled under the keyboard
  assert.equal(Chat.keyboardInset(844, { height: 844.5, offsetTop: 0 }), 0);
});
