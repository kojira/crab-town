// crab-town viewer: viewport math for narrow screens (pure functions, no DOM).
// Loaded as a classic script in the browser (global CrabViewport) and by
// `node --test web/` (module.exports).
const CrabViewport = (() => {
  const MIN_TILE = 16; // smallest readable tile on screen (css px)
  const MAX_TILE = 32; // the canvas' native tile size

  // "fit" shows the whole town (wide screens); "scroll" shows a fixed-scale
  // viewport that can be dragged around (phones in portrait etc.).
  function chooseMode(availW, townW, minTile = MIN_TILE) {
    if (!(availW > 0) || !(townW > 0)) return "fit";
    return availW / townW >= minTile ? "fit" : "scroll";
  }

  // On-screen tile size in scroll mode: fill the available height, but never
  // below minTile (readability) nor above the native size.
  function tileSize(availH, townH, minTile = MIN_TILE, maxTile = MAX_TILE) {
    const t = townH > 0 ? Math.floor(availH / townH) : minTile;
    return Math.max(minTile, Math.min(maxTile, t || minTile));
  }

  // Clamp a scroll offset so the view stays inside the content. When the
  // content is smaller than the view along an axis it is centred (offset <= 0).
  function clamp(off, view, content) {
    if (content <= view) return content === view ? 0 : -Math.floor((view - content) / 2);
    return Math.max(0, Math.min(content - view, off));
  }
  function clampView(vx, vy, viewW, viewH, contentW, contentH) {
    return { x: clamp(vx, viewW, contentW), y: clamp(vy, viewH, contentH) };
  }

  // Offset that puts tile (tx, ty) (its centre; fractional ok) in the middle of the view.
  function centerOn(tx, ty, tile, viewW, viewH, contentW, contentH) {
    const px = (tx + 0.5) * tile, py = (ty + 0.5) * tile;
    return clampView(Math.round(px - viewW / 2), Math.round(py - viewH / 2), viewW, viewH, contentW, contentH);
  }

  function rectCenter(r) { return { x: r.x + (r.w - 1) / 2, y: r.y + (r.h - 1) / 2 }; }

  // The outdoor public zone (the garden): a public zone that belongs to no house.
  function gardenZone(room) {
    const zs = (room && room.zones) || [];
    return zs.find(z => z.id === "garden") || zs.find(z => !z.house && z.visibility === "public") || null;
  }

  // Where the first view looks: the viewer's own actor when it is known and
  // visible, otherwise the garden, otherwise the middle of the town.
  function initialFocus(room, actors, selfId) {
    const a = selfId && actors ? actors[selfId] : null;
    if (a && !a.hidden && a.pos && (!room || a.room === room.id)) return { x: a.pos.x, y: a.pos.y, from: "self" };
    const g = gardenZone(room);
    if (g) return { ...rectCenter(g.rect), from: "garden" };
    if (room) return { x: (room.width - 1) / 2, y: (room.height - 1) / 2, from: "town" };
    return null;
  }

  // Jump buttons, taken from the layout: the viewer (if known and visible),
  // every house (named after its owner) and every outdoor public zone.
  function jumpTargets(room, actors, selfId) {
    if (!room) return [];
    const out = [];
    const me = selfId && actors ? actors[selfId] : null;
    if (me && !me.hidden && me.pos) out.push({ id: "self", label: "自分", x: me.pos.x, y: me.pos.y });
    for (const h of room.houses || []) {
      const owner = actors && actors[h.owner];
      out.push({ id: h.id, label: owner && owner.name ? owner.name + "の家" : h.id, ...rectCenter(h.rect) });
    }
    for (const z of room.zones || []) {
      if (!z.house && z.visibility === "public") out.push({ id: z.id, label: z.name || z.id, ...rectCenter(z.rect) });
    }
    return out;
  }

  // Minimap: pixels per tile so the town fits maxW x maxH.
  function minimapScale(townW, townH, maxW, maxH) {
    return Math.max(1, Math.floor(Math.min(maxW / townW, maxH / townH)));
  }
  // Viewport frame in minimap pixels (off/view in screen px at `tile` px per tile).
  function minimapFrame(off, tile, viewW, viewH, contentW, contentH, mScale) {
    const k = mScale / tile;
    const x = Math.max(0, off.x), y = Math.max(0, off.y);
    return { x: x * k, y: y * k, w: Math.min(viewW, contentW - x) * k, h: Math.min(viewH, contentH - y) * k };
  }
  // Minimap pixel -> town tile.
  function minimapToTile(mx, my, mScale) {
    return { x: Math.floor(mx / mScale), y: Math.floor(my / mScale) };
  }

  return { MIN_TILE, MAX_TILE, chooseMode, tileSize, clampView, centerOn, gardenZone, initialFocus, jumpTargets, minimapScale, minimapFrame, minimapToTile };
})();
if (typeof module !== "undefined") module.exports = CrabViewport;
