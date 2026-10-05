// crab-town viewer: WebSocket client for /world.
function updateStatus() {
  statusEl.textContent = Object.values(actors)
    .map(a => a.hidden ? `${a.name}: (見えない場所にいる)` : `${a.name}: ${a.state}${a.using ? " ("+a.using+")" : ""} @ ${a.pos.x},${a.pos.y}`).join(" / ");
}

function log(s) {
  logEl.textContent = (new Date().toLocaleTimeString() + " " + s + "\n" + logEl.textContent).slice(0, 4000);
}

// ?token=<token> on the page URL is forwarded to /world; the server decides who
// the viewer is from it. No token = public viewer.
function viewerQuery() {
  const v = new URLSearchParams(location.search).get("token");
  return v ? "?token=" + encodeURIComponent(v) : "";
}

function inHidden(r, p) {
  return !!r && (r.hidden_zones || []).some(id => {
    const z = (r.zones || []).find(z => z.id === id);
    return z && p && p.x >= z.rect.x && p.y >= z.rect.y && p.x < z.rect.x + z.rect.w && p.y < z.rect.y + z.rect.h;
  });
}

function connect() {
  const ws = new WebSocket((location.protocol === "https:" ? "wss://" : "ws://") + location.host + "/world" + viewerQuery());
  ws.onmessage = (m) => {
    const ev = JSON.parse(m.data);
    if (ev.type === "snapshot") {
      rooms = {}; actors = {};
      for (const r of ev.rooms) rooms[r.id] = r;
      const r0 = ev.rooms[0];
      if (r0) { cv.width = r0.width * T; cv.height = r0.height * T; }
      for (const a of ev.actors) actors[a.id] = a;
      log("snapshot");
    } else if (ev.type === "actor" && ev.actor) {
      actors[ev.actor.id] = ev.actor;
    } else if (ev.type === "interact") {
      log(`interact: ${ev.actor.name} -> ${ev.furniture.label} (${ev.furniture.function})`);
    } else if (ev.type === "occupancy") {
      const r = rooms[ev.room];
      if (r) { r.in_use = ev.in_use || []; r.hidden_zones = ev.hidden_zones || []; }
      // someone inside a now-hidden zone is not reported again: drop stale positions
      for (const a of Object.values(actors)) if (a.room === ev.room && !a.hidden && inHidden(r, a.pos)) a.hidden = true;
    } else if (ev.type === "say" && ev.actor) {
      // display only: a short-lived bubble over the speaker
      speech[ev.actor.id] = { text: ev.message || "", until: performance.now() + CrabTalk.speechMs(ev.message) };
      log(`say: ${ev.actor.name}: ${ev.message || ""}`);
    } else if (ev.type === "knock") {
      log(`knock: ${ev.by} @ ${ev.room} ${ev.message || ""}`);
    }
    updateStatus();
  };
  ws.onclose = () => { statusEl.textContent = "disconnected, retrying..."; setTimeout(connect, 2000); };
}
requestAnimationFrame(draw);
connect();
