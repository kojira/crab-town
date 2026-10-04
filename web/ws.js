// crab-town viewer: WebSocket client for /world.
function updateStatus() {
  statusEl.textContent = Object.values(actors)
    .map(a => `${a.name}: ${a.state}${a.using ? " ("+a.using+")" : ""} @ ${a.pos.x},${a.pos.y}`).join(" / ");
}

function log(s) {
  logEl.textContent = (new Date().toLocaleTimeString() + " " + s + "\n" + logEl.textContent).slice(0, 4000);
}

function connect() {
  const ws = new WebSocket((location.protocol === "https:" ? "wss://" : "ws://") + location.host + "/world");
  ws.onmessage = (m) => {
    const ev = JSON.parse(m.data);
    if (ev.type === "snapshot") {
      rooms = {}; actors = {};
      for (const r of ev.rooms) rooms[r.id] = r;
      for (const a of ev.actors) actors[a.id] = a;
      log("snapshot");
    } else if (ev.type === "actor" && ev.actor) {
      actors[ev.actor.id] = ev.actor;
    } else if (ev.type === "interact") {
      log(`interact: ${ev.actor.name} -> ${ev.furniture.label} (${ev.furniture.function})`);
    } else if (ev.type === "knock") {
      log(`knock: ${ev.by} @ ${ev.room} ${ev.message || ""}`);
    }
    updateStatus();
  };
  ws.onclose = () => { statusEl.textContent = "disconnected, retrying..."; setTimeout(connect, 2000); };
}
requestAnimationFrame(draw);
connect();
