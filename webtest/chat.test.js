// node --test webtest/*.test.js : the Pages chat (UX audit 2026-10-05).
const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const T = require("../web/chatui.js");
const Chat = require("../web/chatlog.js");
const read = (f) => fs.readFileSync(path.join(__dirname, "../web", f), "utf8");
// source assertions: web/*.js is generated from web-src/*.ts
const src = (f) => fs.readFileSync(path.join(__dirname, "../web-src", f.replace(/\.js$/, ".ts")), "utf8");

test("1: a talk that reached 0 relays fails and the input is kept", () => {
  const o = T.outbox();
  const it = o.add("こんにちは");
  assert.equal(it.status, "sending");
  o.sent(it, "e1", 0);
  assert.equal(it.status, "failed");
  assert.match(it.error, /0/);
  assert.equal(T.mayClear(it), false);
  // nostr.js: the submit handler does not clear the input itself
  const js = src("nostr.js");
  const submit = js.slice(js.indexOf('$("talkForm").onsubmit'));
  assert.doesNotMatch(submit.slice(0, submit.indexOf("};")), /value = ""/);
  assert.doesNotMatch(js, /sent \$\{cmd\.type\} to \$\{n\} relay/);
});

test("1/3: sending -> delivered on talk: ok; NG fails with the reason", () => {
  const o = T.outbox();
  const a = o.add("a"), b = o.add("b");
  o.sent(a, "ea", 3); o.sent(b, "eb", 2);
  assert.equal(a.status, "sending");
  assert.equal(T.mayClear(a), false);
  assert.equal(o.result("ea", true), a);
  assert.equal(a.status, "delivered");
  assert.equal(T.mayClear(a), true);
  assert.equal(o.result("ea", true), null); // a second relay's copy does nothing
  o.result("eb", false, "forbidden");
  assert.equal(b.status, "failed");
  assert.equal(b.error, "forbidden");
  assert.equal(o.result("zz", true), null);
  // the echoed talk confirms it too (before the result)
  const c = o.add("c"); o.sent(c, "ec", 1);
  assert.equal(o.echo("c"), c);
  assert.equal(c.status, "delivered");
  assert.equal(o.result("ec", true), null);
  assert.deepEqual(T.STATUS_TEXT, { sending: "送信中…", delivered: "届いた", failed: "送れなかった" });
});

test("2: without NIP-07 the input says how to get to talk", () => {
  assert.match(T.inputHint(false), /NIP-07/);
  assert.match(T.inputHint(false), /ログインすると話せる/);
  assert.doesNotMatch(T.inputHint(true), /NIP-07/);
  const html = read("nostr.html");
  assert.match(html, /<input id="talkText"[^>]*placeholder="NIP-07[^"]*ログインすると話せる"[^>]*disabled>/);
  assert.match(html, /id="talkHint"/);
  // no throwaway key for talking
  assert.doesNotMatch(src("nostr.js"), /type: "talk"[^\n]*anonKey/);
});

test("4: history is drawn oldest first, with the event's time", () => {
  const got = T.sortBacklog([
    { type: "say", created_at: 30, message: "3" },
    { type: "talk", created_at: 20, message: "2" },
    { type: "say", created_at: 20, message: "2b" },
    { type: "talk", created_at: 10, message: "1" },
  ]).map(x => x.message);
  assert.deepEqual(got, ["1", "2", "2b", "3"]);
  const e = Chat.entry({ type: "talk", by: "nostr:x", message: "hi", at: 1700000000 }, {});
  assert.equal(e.at, 1700000000);
  assert.match(src("chatlog.js"), /new Date\(at \* 1000\)/);
});

test("5: only refused commands get a line; ok goes into the status", () => {
  assert.equal(T.resultWorthALine({ type: "result", cmd: "talk", ok: true }), false);
  assert.equal(T.resultWorthALine({ type: "result", cmd: "snapshot", ok: true }), false);
  assert.equal(T.resultWorthALine({ type: "result", cmd: "move", ok: false, error: "forbidden" }), true);
});

test("6: a say answering me reads '→ あなた'; others' replies are dimmed", () => {
  const me = { selfGuestId: "nostr:aaaaaaaaaaaaaaaa" };
  const say = (reply_to) => Chat.entry({ type: "say", actor: { id: "nostarou", name: "のすたろう" }, message: "よっ", reply_to }, me);
  assert.equal(say("nostr:aaaaaaaaaaaaaaaa").to, "あなた");
  assert.ok(!say("nostr:aaaaaaaaaaaaaaaa").dim);
  assert.ok(say("nostr:bbbbbbbbbbbbbbbb").dim);
  assert.ok(!say("").dim);
  assert.equal(T.addressed({ reply_to: "nostr:aaaaaaaaaaaaaaaa" }, me.selfGuestId), "you");
  assert.equal(T.addressed({ reply_to: "nostr:b" }, me.selfGuestId), "other");
  assert.equal(T.addressed({}, me.selfGuestId), "none");
  // another visitor's talk is dimmed too, mine is not
  assert.ok(Chat.entry({ type: "talk", by: "nostr:cccccccccccccccc", message: "x" }, me).dim);
  assert.ok(!Chat.entry({ type: "talk", by: "nostr:aaaaaaaaaaaaaaaa", message: "x" }, me).dim);
  // the reply_to tag of the state event is read
  assert.match(src("nostr.js"), /tag\("reply_to"\)/);
});

test("7: phone log keeps its room while typing, new-lines button, toggle says what it does", () => {
  const html = read("nostr.html");
  const css = html.slice(html.indexOf("<style>"), html.indexOf("</style>"));
  assert.match(css, /body\.narrow\.kbup [^{]*#chatHead[^{]*\{[^}]*display:none/); // keyboard up: rows above the log fold away
  assert.match(html, /id="newLines"[^>]*hidden>↓ 新着</);
  assert.equal(Chat.toggleLabel(true), "▲ ログを開く");
  assert.equal(Chat.toggleLabel(false), "▼ ログを閉じる");
  assert.doesNotMatch(html, />開閉</);
});

test("8: bubbles wrap over lines and stay longer for longer text", () => {
  const m = (s) => [...s].length * 10; // 10px a character
  const long = "あ".repeat(70);
  const lines = T.wrap(long, 200, m);
  assert.equal(lines.length, 4);
  assert.equal(lines.join(""), long); // nothing cut at 40
  assert.ok(lines.every(l => m(l) <= 200));
  assert.deepEqual(T.wrap("a\nb", 200, m), ["a", "b"]);
  assert.ok(T.wrap("あ".repeat(500), 100, m, 3).at(-1).endsWith("…"));
  assert.equal(T.speechMs("よっ"), T.SPEECH_MIN_MS);
  assert.ok(T.speechMs("あ".repeat(140)) > T.speechMs("あ".repeat(40)));
  assert.ok(T.speechMs("あ".repeat(140)) >= 20000);
  assert.ok(T.speechMs("あ".repeat(5000)) <= T.SPEECH_MAX_MS);
  assert.doesNotMatch(src("render.js"), /slice\(0, 40\)/);
});
