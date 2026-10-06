// crab-town viewer: viewport math for narrow screens (pure functions, no DOM).
// Loaded as a classic script in the browser (global CrabViewport) and by
// `node --test web/` (module.exports).
const CrabViewport = (() => {
  const MIN_TILE = 16; // smallest readable tile on screen (css px)
  const MAX_TILE = 32; // the canvas' native tile size
  const SCROLL_MIN_TILE = 24; // scroll mode on phones: big enough to read; the view scrolls both ways

  // "fit" shows the whole town (wide screens); "scroll" shows a fixed-scale
  // viewport that can be dragged around (phones in portrait etc.).
  function chooseMode(availW: number, townW: number, minTile = MIN_TILE): "fit" | "scroll" {
    if (!(availW > 0) || !(townW > 0)) return "fit";
    return availW / townW >= minTile ? "fit" : "scroll";
  }

  // On-screen tile size in scroll mode: fill the available height, but never
  // below minTile (readability) nor above the native size.
  function tileSize(availH: number, townH: number, minTile = MIN_TILE, maxTile = MAX_TILE) {
    const t = townH > 0 ? Math.floor(availH / townH) : minTile;
    return Math.max(minTile, Math.min(maxTile, t || minTile));
  }

  // Scroll-mode tile on phones: fill the height when it can, but never below
  // SCROLL_MIN_TILE so tiles and canvas text stay readable (the stage then
  // scrolls vertically too). Fit mode is unaffected.
  function scrollTile(availH: number, townH: number) {
    return tileSize(availH, townH, SCROLL_MIN_TILE, MAX_TILE);
  }

  // Height of the map stage: everything between its top and what must stay
  // visible under it (status lines + the fixed chat panel), so no empty band
  // is left between the map and the chat. Never below minH.
  function stageHeight(innerH: number, top: number, below: number, chatH: number, minH = 160) {
    const h = Math.floor(innerH - top - below - chatH);
    return Math.max(minH, h > 0 ? h : 0);
  }

  // Clamp a scroll offset so the view stays inside the content. When the
  // content is smaller than the view along an axis it is centred (offset <= 0).
  function clamp(off: number, view: number, content: number) {
    if (content <= view) return content === view ? 0 : -Math.floor((view - content) / 2);
    return Math.max(0, Math.min(content - view, off));
  }
  function clampView(vx: number, vy: number, viewW: number, viewH: number, contentW: number, contentH: number): Pos {
    return { x: clamp(vx, viewW, contentW), y: clamp(vy, viewH, contentH) };
  }

  // Offset that puts tile (tx, ty) (its centre; fractional ok) in the middle of the view.
  function centerOn(tx: number, ty: number, tile: number, viewW: number, viewH: number, contentW: number, contentH: number): Pos {
    const px = (tx + 0.5) * tile, py = (ty + 0.5) * tile;
    return clampView(Math.round(px - viewW / 2), Math.round(py - viewH / 2), viewW, viewH, contentW, contentH);
  }

  function rectCenter(r: Rect): Pos { return { x: r.x + (r.w - 1) / 2, y: r.y + (r.h - 1) / 2 }; }

  // The outdoor public zone (the garden): a public zone that belongs to no house.
  function gardenZone(room: Room | null | undefined): Zone | null {
    const zs = (room && room.zones) || [];
    return zs.find(z => z.id === "garden") || zs.find(z => !z.house && z.visibility === "public") || null;
  }

  // Where the first view looks: the viewer's own actor when it is known and
  // visible, otherwise the garden, otherwise the middle of the town.
  function initialFocus(room: Room | null | undefined, actors: Record<string, Actor> | null | undefined, selfId?: string | null) {
    const a = selfId && actors ? actors[selfId] : null;
    if (a && !a.hidden && a.pos && (!room || a.room === room.id)) return { x: a.pos.x, y: a.pos.y, from: "self" };
    const g = gardenZone(room);
    if (g) return { ...rectCenter(g.rect), from: "garden" };
    if (room) return { x: (room.width - 1) / 2, y: (room.height - 1) / 2, from: "town" };
    return null;
  }

  // Jump buttons, taken from the layout: the viewer (if known and visible),
  // every house (named after its owner) and every outdoor public zone.
  function jumpTargets(room: Room | null | undefined, actors: Record<string, Actor> | null | undefined, selfId?: string | null) {
    if (!room) return [];
    const out: { id: string; label: string; x: number; y: number }[] = [];
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

  // Canvas labels (name tags, tooltips, speech) are drawn in canvas px at the
  // native tile size T. When a tile is shown at `shownTile` css px they shrink;
  // this factor keeps a label of basePx canvas px at least minCssPx on screen.
  function labelScale(shownTile: number, nativeTile = MAX_TILE, basePx = 11, minCssPx = 13) {
    if (!(shownTile > 0) || !(nativeTile > 0)) return 1;
    return Math.max(1, (minCssPx * nativeTile) / (basePx * shownTile));
  }

  // Knock targets: every house, named after its owner. reachable = someone is
  // listening there (its owner is in `listening`, e.g. the actor connected via
  // extgate); otherwise a knock is only recorded as a town event.
  function knockTargets(room: Room | null | undefined, actors: Record<string, Actor> | null | undefined, listening: string[] = []) {
    return ((room && room.houses) || []).map(h => {
      const o = actors && actors[h.owner];
      return { id: h.id, owner: h.owner, ownerName: o && o.name ? o.name : h.owner, label: (o && o.name ? o.name : h.owner) + "の家", reachable: listening.includes(h.owner) };
    });
  }

  // Minimap: pixels per tile so the town fits maxW x maxH.
  function minimapScale(townW: number, townH: number, maxW: number, maxH: number) {
    return Math.max(1, Math.floor(Math.min(maxW / townW, maxH / townH)));
  }
  // Viewport frame in minimap pixels (off/view in screen px at `tile` px per tile).
  function minimapFrame(off: Pos, tile: number, viewW: number, viewH: number, contentW: number, contentH: number, mScale: number) {
    const k = mScale / tile;
    const x = Math.max(0, off.x), y = Math.max(0, off.y);
    return { x: x * k, y: y * k, w: Math.min(viewW, contentW - x) * k, h: Math.min(viewH, contentH - y) * k };
  }
  // Minimap pixel -> town tile.
  function minimapToTile(mx: number, my: number, mScale: number): Pos {

    return { x: Math.floor(mx / mScale), y: Math.floor(my / mScale) };
  }

  return { MIN_TILE, MAX_TILE, SCROLL_MIN_TILE, labelScale, scrollTile, stageHeight, knockTargets, chooseMode, tileSize, clampView, centerOn, gardenZone, initialFocus, jumpTargets, minimapScale, minimapFrame, minimapToTile };
})();
if (typeof module !== "undefined") module.exports = CrabViewport;
