// node --test webtest/*.test.js : iOS Safari zooms the page when a focused
// input / select / textarea has text under 16px. Every rule that sets a
// form control's font size must keep it >= 16px, and zooming must stay allowed.
const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");

const html = fs.readFileSync(path.join(__dirname, "../web/nostr.html"), "utf8");
const css = html.slice(html.indexOf("<style>") + 7, html.indexOf("</style>")).replace(/\/\*[\s\S]*?\*\//g, "");
const rules = [...css.matchAll(/([^{}]+)\{([^}]*)\}/g)].map(m => ({ sel: m[1].trim(), body: m[2] }));
// the font size a rule gives (font-size or the font shorthand), in px; null = none
function px(body) {
  const m = body.match(/font-size:\s*([\d.]+)px/) || body.match(/(?:^|;)\s*font:[^;]*?([\d.]+)px/);
  return m ? parseFloat(m[1]) : null;
}
const controls = ["talkText", "knockTo"]; // the page's input and select
const targets = (sel) => sel.split(",").map(s => s.trim()).some(s =>
  /(^|[\s>])(input|select|textarea)\b/.test(s) || controls.some(id => s.includes("#" + id)));

test("form controls are never below 16px (no iOS focus zoom)", () => {
  const set = rules.filter(r => targets(r.sel) && px(r.body) !== null);
  assert.ok(set.some(r => /(^|,)\s*input\b/.test(r.sel) && px(r.body) >= 16), "a base rule sets inputs to >= 16px");
  for (const r of set) assert.ok(px(r.body) >= 16, `${r.sel} sets ${px(r.body)}px`);
});

test("zoom is not disabled to hide it", () => {
  const vp = html.match(/<meta name="viewport" content="([^"]*)"/)[1];
  assert.doesNotMatch(vp, /maximum-scale|user-scalable/);
});
