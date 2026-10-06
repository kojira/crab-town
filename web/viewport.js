// generated from web-src/viewport.ts by scripts/build-web.mjs; edit the .ts file, not this one
const CrabViewport = /* @__PURE__ */ (() => {
  const MIN_TILE = 16;
  const MAX_TILE = 32;
  const SCROLL_MIN_TILE = 24;
  function chooseMode(availW, townW, minTile = MIN_TILE) {
    if (!(availW > 0) || !(townW > 0)) return "fit";
    return availW / townW >= minTile ? "fit" : "scroll";
  }
  function tileSize(availH, townH, minTile = MIN_TILE, maxTile = MAX_TILE) {
    const t = townH > 0 ? Math.floor(availH / townH) : minTile;
    return Math.max(minTile, Math.min(maxTile, t || minTile));
  }
  function scrollTile(availH, townH) {
    return tileSize(availH, townH, SCROLL_MIN_TILE, MAX_TILE);
  }
  function stageHeight(innerH, top, below, chatH, minH = 160) {
    const h = Math.floor(innerH - top - below - chatH);
    return Math.max(minH, h > 0 ? h : 0);
  }
  function clamp(off, view, content) {
    if (content <= view) return content === view ? 0 : -Math.floor((view - content) / 2);
    return Math.max(0, Math.min(content - view, off));
  }
  function clampView(vx, vy, viewW, viewH, contentW, contentH) {
    return { x: clamp(vx, viewW, contentW), y: clamp(vy, viewH, contentH) };
  }
  function centerOn(tx, ty, tile, viewW, viewH, contentW, contentH) {
    const px = (tx + 0.5) * tile, py = (ty + 0.5) * tile;
    return clampView(Math.round(px - viewW / 2), Math.round(py - viewH / 2), viewW, viewH, contentW, contentH);
  }
  function rectCenter(r) {
    return { x: r.x + (r.w - 1) / 2, y: r.y + (r.h - 1) / 2 };
  }
  function gardenZone(room) {
    const zs = room && room.zones || [];
    return zs.find((z) => z.id === "garden") || zs.find((z) => !z.house && z.visibility === "public") || null;
  }
  function initialFocus(room, actors, selfId) {
    const a = selfId && actors ? actors[selfId] : null;
    if (a && !a.hidden && a.pos && (!room || a.room === room.id)) return { x: a.pos.x, y: a.pos.y, from: "self" };
    const g = gardenZone(room);
    if (g) return { ...rectCenter(g.rect), from: "garden" };
    if (room) return { x: (room.width - 1) / 2, y: (room.height - 1) / 2, from: "town" };
    return null;
  }
  function jumpTargets(room, actors, selfId) {
    if (!room) return [];
    const out = [];
    const me = selfId && actors ? actors[selfId] : null;
    if (me && !me.hidden && me.pos) out.push({ id: "self", label: "\u81EA\u5206", x: me.pos.x, y: me.pos.y });
    for (const h of room.houses || []) {
      const owner = actors && actors[h.owner];
      out.push({ id: h.id, label: owner && owner.name ? owner.name + "\u306E\u5BB6" : h.id, ...rectCenter(h.rect) });
    }
    for (const z of room.zones || []) {
      if (!z.house && z.visibility === "public") out.push({ id: z.id, label: z.name || z.id, ...rectCenter(z.rect) });
    }
    return out;
  }
  function labelScale(shownTile, nativeTile = MAX_TILE, basePx = 11, minCssPx = 13) {
    if (!(shownTile > 0) || !(nativeTile > 0)) return 1;
    return Math.max(1, minCssPx * nativeTile / (basePx * shownTile));
  }
  function knockTargets(room, actors, listening = []) {
    return (room && room.houses || []).map((h) => {
      const o = actors && actors[h.owner];
      return { id: h.id, owner: h.owner, ownerName: o && o.name ? o.name : h.owner, label: (o && o.name ? o.name : h.owner) + "\u306E\u5BB6", reachable: listening.includes(h.owner) };
    });
  }
  function minimapScale(townW, townH, maxW, maxH) {
    return Math.max(1, Math.floor(Math.min(maxW / townW, maxH / townH)));
  }
  function minimapFrame(off, tile, viewW, viewH, contentW, contentH, mScale) {
    const k = mScale / tile;
    const x = Math.max(0, off.x), y = Math.max(0, off.y);
    return { x: x * k, y: y * k, w: Math.min(viewW, contentW - x) * k, h: Math.min(viewH, contentH - y) * k };
  }
  function minimapToTile(mx, my, mScale) {
    return { x: Math.floor(mx / mScale), y: Math.floor(my / mScale) };
  }
  return { MIN_TILE, MAX_TILE, SCROLL_MIN_TILE, labelScale, scrollTile, stageHeight, knockTargets, chooseMode, tileSize, clampView, centerOn, gardenZone, initialFocus, jumpTargets, minimapScale, minimapFrame, minimapToTile };
})();
if (typeof module !== "undefined") module.exports = CrabViewport;
