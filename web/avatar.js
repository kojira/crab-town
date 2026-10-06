// generated from web-src/avatar.ts by scripts/build-web.mjs; edit the .ts file, not this one
const CrabAvatar = /* @__PURE__ */ (() => {
  const profiles = {};
  function parseProfile(ev) {
    let c = {};
    try {
      c = JSON.parse(ev && ev.content || "{}") || {};
    } catch {
      c = {};
    }
    const s = (v) => typeof v === "string" ? v.trim() : "";
    const pic = s(c.picture);
    return { name: s(c.display_name) || s(c.name), picture: /^https?:\/\//i.test(pic) ? pic : "" };
  }
  function pickLatest(events, pubkey) {
    let best = null;
    for (const ev of events || []) {
      if (!ev || ev.kind !== 0 || ev.pubkey !== pubkey) continue;
      if (!best || ev.created_at > best.created_at) best = ev;
    }
    return best;
  }
  function initial(name, pubkey) {
    const ch = [...String(name || "").trim()][0] || String(pubkey || "?")[0] || "?";
    return ch.toUpperCase();
  }
  function hue(pubkey) {
    let h = 0;
    for (const c of String(pubkey || "")) h = (h * 31 + c.charCodeAt(0)) % 360;
    return h;
  }
  function mode(p) {
    return p && p.status === "image" && p.img ? "image" : "initial";
  }
  function fetchProfile(pubkey, relays, verify, timeoutMs = 5e3) {
    return new Promise((resolve) => {
      const got = [], socks = [];
      let left = relays.length, done = false;
      const finish = () => {
        if (done) return;
        done = true;
        for (const ws of socks) try {
          ws.close();
        } catch {
        }
        resolve(pickLatest(got.filter((ev) => {
          try {
            return verify(ev);
          } catch {
            return false;
          }
        }), pubkey));
      };
      const oneDone = () => {
        if (--left <= 0) finish();
      };
      if (!left) return finish();
      setTimeout(finish, timeoutMs);
      for (const url of relays) {
        let ws, ended = false;
        const end = () => {
          if (!ended) {
            ended = true;
            oneDone();
          }
        };
        try {
          ws = new WebSocket(url);
        } catch {
          end();
          continue;
        }
        socks.push(ws);
        ws.onopen = () => ws.send(JSON.stringify(["REQ", "crab-k0", { kinds: [0], authors: [pubkey], limit: 1 }]));
        ws.onmessage = (m) => {
          let d;
          try {
            d = JSON.parse(m.data);
          } catch {
            return;
          }
          if (d[0] === "EVENT" && d[2]) got.push(d[2]);
          else if (d[0] === "EOSE") end();
        };
        ws.onerror = end;
        ws.onclose = end;
      }
    });
  }
  function load(pubkey, relays, verify) {
    if (!pubkey || profiles[pubkey]) return profiles[pubkey];
    const p = profiles[pubkey] = { name: "", picture: "", img: null, status: "loading" };
    fetchProfile(pubkey, relays, verify).then((ev) => {
      Object.assign(p, ev ? parseProfile(ev) : {});
      if (!p.picture) {
        p.status = "initial";
        return;
      }
      const img = new Image();
      img.referrerPolicy = "no-referrer";
      img.onload = () => {
        p.img = img;
        p.status = "image";
      };
      img.onerror = () => {
        p.status = "initial";
      };
      img.src = p.picture;
    }, () => {
      p.status = "initial";
    });
    return p;
  }
  function draw(ctx, a, x0, y0, t) {
    const p = profiles[a.pubkey || ""];
    const r = t / 2 - 1, cx = x0 + t / 2, cy = y0 + t / 2;
    ctx.save();
    ctx.beginPath();
    ctx.arc(cx, cy, r, 0, Math.PI * 2);
    ctx.closePath();
    if (mode(p) === "image" && p.img) {
      ctx.clip();
      ctx.imageSmoothingEnabled = true;
      ctx.drawImage(p.img, cx - r, cy - r, r * 2, r * 2);
    } else {
      ctx.fillStyle = `hsl(${hue(a.pubkey)},45%,42%)`;
      ctx.fill();
      ctx.fillStyle = "#fff";
      ctx.font = `bold ${Math.round(t * 0.5)}px sans-serif`;
      ctx.textAlign = "center";
      ctx.textBaseline = "middle";
      ctx.fillText(initial(p && p.name || a.name, a.pubkey), cx, cy + 1);
    }
    ctx.restore();
    ctx.beginPath();
    ctx.arc(cx, cy, r, 0, Math.PI * 2);
    ctx.strokeStyle = "#ffd84a";
    ctx.lineWidth = 2;
    ctx.stroke();
  }
  const label = (a) => a.pubkey && profiles[a.pubkey] && profiles[a.pubkey].name || a.name;
  return { parseProfile, pickLatest, initial, hue, mode, fetchProfile, load, draw, label, profiles };
})();
if (typeof module !== "undefined") module.exports = CrabAvatar;
