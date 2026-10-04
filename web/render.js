// crab-town viewer: canvas rendering (floor, furniture, actors, tooltips).
function drawSprite(rows, pal, x0, y0, flip) {
  for (let y = 0; y < rows.length; y++) {
    const row = rows[y];
    for (let x = 0; x < row.length; x++) {
      const c = pal[row[x]];
      if (!c) continue;
      const dx = flip ? row.length - 1 - x : x;
      ctx.fillStyle = c;
      ctx.fillRect(x0 + dx * P, y0 + y * P, P, P);
    }
  }
}

// ---- labels -----------------------------------------------------------------
// Shrink the font until text fits maxW; returns the px size used.
function fitFont(text, maxW, maxPx, minPx) {
  for (let px = maxPx; px > minPx; px--) {
    ctx.font = `bold ${px}px sans-serif`;
    if (ctx.measureText(text).width <= maxW) return px;
  }
  ctx.font = `bold ${minPx}px sans-serif`;
  return minPx;
}

// Furniture label: not drawn by default (sprites must be readable on their own).
// Shown only as a hover tooltip, placed above/below the tile and clamped to the canvas.
let hover = null; // furniture under the mouse
function drawTooltip(f) {
  const s = furnitureSize(f);
  const px = fitFont(f.label, T * 5, 13, 9);
  const w = Math.ceil(ctx.measureText(f.label).width) + 8, h = px + 6;
  const x = Math.max(0, Math.min(cv.width - w, Math.round(f.pos.x * T + s.w * T / 2 - w / 2)));
  let y = f.pos.y * T - h - 2;                 // above the piece
  if (y < 0) y = (f.pos.y + s.h) * T + 2;      // no room above: below it
  y = Math.max(0, Math.min(cv.height - h, y));
  ctx.fillStyle = "rgba(10,10,14,0.85)"; ctx.fillRect(x, y, w, h);
  ctx.strokeStyle = "#ffd84a"; ctx.lineWidth = 1; ctx.strokeRect(x + 0.5, y + 0.5, w - 1, h - 1);
  ctx.fillStyle = "#fff"; ctx.textAlign = "center"; ctx.textBaseline = "middle";
  ctx.fillText(f.label, x + w / 2, y + h / 2 + 0.5);
  ctx.textBaseline = "alphabetic";
}
cv.addEventListener("mousemove", (e) => {
  const r = cv.getBoundingClientRect();
  const tx = Math.floor((e.clientX - r.left) * cv.width / r.width / T);
  const ty = Math.floor((e.clientY - r.top) * cv.height / r.height / T);
  const room = Object.values(rooms)[0];
  hover = room ? room.furniture.find(f => !f.walkable && occupies(f, tx, ty)) || room.furniture.find(f => occupies(f, tx, ty)) || null : null;
  cv.style.cursor = hover ? "help" : "default";
});
cv.addEventListener("mouseleave", () => { hover = null; });

// Name tag below the actor (above it near the bottom edge), clamped to the canvas.
function drawNameTag(text, cx, topY, botY) {
  const px = fitFont(text, T * 3, 11, 8);
  const w = Math.ceil(ctx.measureText(text).width) + 6, h = px + 4;
  const x = Math.max(0, Math.min(cv.width - w, Math.round(cx - w / 2)));
  let y = botY + 1;
  if (y + h > cv.height) y = topY - h - 1;
  y = Math.max(0, y);
  ctx.fillStyle = "rgba(10,10,14,0.75)";
  ctx.fillRect(x, y, w, h);
  ctx.fillStyle = "#ffd84a";
  ctx.textAlign = "center"; ctx.textBaseline = "middle";
  ctx.fillText(text, x + w / 2, y + h / 2 + 0.5);
  ctx.textBaseline = "alphabetic";
}

// Small speech-bubble state icon in the actor tile's top-right corner.
function drawStateIcon(state, x0, y0) {
  const icon = { talking:"...", working:"#", away:"z" }[state];
  if (!icon) return;
  const w = 12, h = 9, x = x0 + T - w, y = y0;
  ctx.fillStyle = "#fff"; ctx.fillRect(x, y, w, h);
  ctx.fillRect(x + 1, y + h, 2, 2);
  ctx.fillStyle = "#111"; ctx.font = "bold 8px sans-serif";
  ctx.textAlign = "center"; ctx.textBaseline = "middle";
  ctx.fillText(icon, x + w / 2, y + h / 2);
  ctx.textBaseline = "alphabetic";
}

// ---- drawing ----------------------------------------------------------------
function lerpPos(v, now) {
  const t = Math.min(1, (now - v.start) / STEP_MS);
  return { x: v.from.x + (v.to.x - v.from.x) * t, y: v.from.y + (v.to.y - v.from.y) * t };
}
function actorView(a, now) {
  let v = view[a.id];
  if (!v) v = view[a.id] = { from: { ...a.pos }, to: { ...a.pos }, start: now, flip: false, frame: 0 };
  if (v.to.x !== a.pos.x || v.to.y !== a.pos.y) {
    const cur = lerpPos(v, now);
    if (a.pos.x !== v.to.x) v.flip = a.pos.x < v.to.x;
    v.from = cur; v.to = { ...a.pos }; v.start = now; v.frame ^= 1;
  }
  return v;
}

function drawActor(a, now) {
  const v = actorView(a, now);
  const p = lerpPos(v, now);
  const moving = p.x !== v.to.x || p.y !== v.to.y || !!a.target;
  const x0 = Math.round(p.x * T), y0 = Math.round(p.y * T);
  if (a.target) {
    ctx.strokeStyle = "#ffd84a88"; ctx.setLineDash([3, 3]);
    ctx.strokeRect(a.target.x * T + 3.5, a.target.y * T + 3.5, T - 7, T - 7);
    ctx.setLineDash([]);
  }
  ctx.fillStyle = "rgba(0,0,0,0.35)"; // shadow
  ctx.fillRect(x0 + 4 * P, y0 + 15 * P, 8 * P, P);
  const bob = moving && v.frame ? -P : 0;
  const sp = spriteFor(a);
  drawSprite(sp.body, sp.pal, x0, y0 + bob, v.flip);
  drawSprite(sp.legs[moving ? v.frame : 0], sp.pal, x0, y0 + 13 * P, v.flip);
  drawStateIcon(a.state, x0, y0);
  drawNameTag(a.name, x0 + T / 2, y0, y0 + T);
}

function draw() {
  const now = performance.now();
  const room = Object.values(rooms)[0];
  ctx.clearRect(0, 0, cv.width, cv.height);
  if (room) {
    drawFloor(room);
    drawWalls(room);
    // flat props (rugs, mats) first so everything else stands on them
    for (const f of room.furniture) if (f.walkable) drawFurniture(f);
    for (const f of room.furniture) if (!f.walkable) drawFurniture(f);
    drawHiddenZones(room);
    drawUseLamps(room);
    // hidden actors (in zones this viewer may not see) are not drawn at all
    for (const a of Object.values(actors)) if (a.room === room.id && !a.hidden) drawActor(a, now);
    if (hover) drawTooltip(hover);
  }
  requestAnimationFrame(draw);
}
