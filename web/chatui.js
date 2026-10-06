// generated from web-src/chatui.ts by scripts/build-web.mjs; edit the .ts file, not this one
const CrabTalk = /* @__PURE__ */ (() => {
  const STATUS_TEXT = { sending: "\u9001\u4FE1\u4E2D\u2026", delivered: "\u5C4A\u3044\u305F", failed: "\u9001\u308C\u306A\u304B\u3063\u305F" };
  function outbox() {
    const items = [];
    let seq = 0;
    return {
      items,
      add(text, image) {
        const it = { key: ++seq, text, status: "sending", eventId: "", error: "", image };
        items.push(it);
        return it;
      },
      // after publishing: n relays took the event. 0 = nothing left the page.
      sent(it, eventId, n) {
        it.eventId = eventId || "";
        if (!(n > 0)) {
          it.status = "failed";
          it.error = "\u3064\u306A\u304C\u3063\u3066\u3044\u308B\u30EA\u30EC\u30FC\u304C\u7121\u3044\uFF080\u672C\uFF09";
        }
        return it;
      },
      signFailed(it, err) {
        it.status = "failed";
        it.error = String(err || "\u7F72\u540D\u3067\u304D\u306A\u304B\u3063\u305F");
        return it;
      },
      // the town's result for one of our events (matched by its e tag)
      result(eventId, ok, error) {
        const it = items.find((x) => x.eventId && x.eventId === eventId);
        if (!it || it.status === "delivered") return null;
        if (ok) it.status = "delivered";
        else {
          it.status = "failed";
          it.error = error || "\u753A\u304C\u53D7\u3051\u4ED8\u3051\u306A\u304B\u3063\u305F";
        }
        return it;
      },
      // the town echoing a talk event by us with this text: it arrived
      echo(text, image) {
        const it = items.find((x) => x.text === text && (x.image || "") === (image || "") && x.status !== "failed" && !x.echoed);
        if (!it) return null;
        it.echoed = true;
        it.status = "delivered";
        return it;
      },
      remove(it) {
        const i = items.indexOf(it);
        if (i >= 0) items.splice(i, 1);
      }
    };
  }
  const mayClear = (it) => !!it && it.status === "delivered";
  const RANK = { snapshot: 0, actor: 1, occupancy: 1, knock: 2, talk: 2, result: 3, say: 4 };
  function sortBacklog(list) {
    return list.map((x, i) => ({ x, i })).sort((a, b) => a.x.created_at - b.x.created_at || (RANK[a.x.type] ?? 1) - (RANK[b.x.type] ?? 1) || a.i - b.i).map((o) => o.x);
  }
  const resultWorthALine = (ev) => !!ev && ev.type === "result" && !ev.ok;
  function addressed(ev, selfId) {
    if (!ev || !ev.reply_to) return "none";
    return selfId && ev.reply_to === selfId ? "you" : "other";
  }
  function wrap(text, maxW, measure, maxLines = Infinity) {
    const out = [];
    for (const para of String(text || "").split("\n")) {
      let line = "";
      for (const ch of para) {
        if (line && measure(line + ch) > maxW) {
          out.push(line);
          line = "";
        }
        line += ch;
      }
      out.push(line);
    }
    if (out.length > maxLines) {
      out.length = maxLines;
      out[maxLines - 1] = out[maxLines - 1].slice(0, -1) + "\u2026";
    }
    return out;
  }
  const SPEECH_MIN_MS = 6e3, SPEECH_MAX_MS = 6e4;
  function speechMs(text) {
    const n = [...String(text || "")].length;
    return Math.max(SPEECH_MIN_MS, Math.min(SPEECH_MAX_MS, 3e3 + n * 125));
  }
  const HINT_LOGIN = "NIP-07 \u62E1\u5F35\uFF08nos2x\u30FBAlby \u306A\u3069\uFF09\u3067\u30ED\u30B0\u30A4\u30F3\u3059\u308B\u3068\u8A71\u305B\u308B";
  const HINT_TALK = "\u306E\u3059\u305F\u308D\u3046\u306B\u8A71\u3057\u304B\u3051\u308B\uFF08\u5E73\u6587\u30FB\u516C\u958B\u30FB280\u5B57\u307E\u3067\uFF09";
  const inputHint = (loggedIn) => loggedIn ? HINT_TALK : HINT_LOGIN;
  return {
    STATUS_TEXT,
    outbox,
    mayClear,
    sortBacklog,
    resultWorthALine,
    addressed,
    wrap,
    speechMs,
    SPEECH_MIN_MS,
    SPEECH_MAX_MS,
    inputHint,
    HINT_LOGIN
  };
})();
if (typeof module !== "undefined") module.exports = CrabTalk;
