// crab-town viewer: floors (one texture per zone), walls and doors.
// Floor painters draw one tile at (px, py); d(x, y, w, h, c) is in dots.
const FLOORS: Record<string, FloorPainter> = {
  // 玄関: grey stone slabs
  stone(d, x, y) {
    d(0, 0, 16, 16, (x + y) % 2 ? "#8c8c88" : "#80807c");
    d(0, 15, 16, 1, "#5f5f5c"); d(15, 0, 1, 16, "#5f5f5c"); d(3, 4, 2, 1, "#9a9a96"); d(10, 10, 2, 1, "#74746f");
  },
  // キッチン: white / mint checker tiles with grout
  tile(d, x, y) {
    d(0, 0, 16, 16, (x + y) % 2 ? "#e9f3ef" : "#a9d8c8");
    d(0, 0, 16, 1, "#c9d2ce"); d(0, 0, 1, 16, "#c9d2ce");
  },
  // ダイニング: light oak planks
  wood(d, x, y) {
    d(0, 0, 16, 16, "#b98a56"); d(0, 7, 16, 1, "#946a3c"); d(0, 15, 16, 1, "#946a3c");
    d(y % 2 ? 4 : 12, 0, 1, 7, "#946a3c"); d(y % 2 ? 10 : 2, 8, 1, 7, "#946a3c"); d(6, 3, 3, 1, "#c99a66");
  },
  // 廊下: darker long planks
  hall(d, x, y) {
    d(0, 0, 16, 16, "#9a6e42"); d(0, 0, 16, 1, "#7a5430"); d(0, 8, 16, 1, "#7a5430");
    d((x * 5) % 16, 1, 1, 7, "#7a5430"); d((x * 5 + 8) % 16, 9, 1, 7, "#7a5430");
  },
  // リビング: wood floor with a big patterned rug in the middle
  rug(d, x, y, z) {
    FLOORS.wood(d, x, y, z);
    const r = z.rect, inRug = x > r.x && x < r.x + r.w - 2 && y > r.y && y < r.y + r.h - 1;
    if (!inRug) return;
    d(0, 0, 16, 16, "#b0413e");
    if (x === r.x + 1) d(0, 0, 3, 16, "#e8c070");
    if (x === r.x + r.w - 3) d(13, 0, 3, 16, "#e8c070");
    if (y === r.y + 1) d(0, 0, 16, 3, "#e8c070");
    if (y === r.y + r.h - 2) d(0, 13, 16, 3, "#e8c070");
    if ((x + y) % 2) d(6, 6, 4, 4, "#e8c070"); else { d(7, 3, 2, 10, "#8a2c2a"); d(3, 7, 10, 2, "#8a2c2a"); }
  },
  // 寝室: soft lavender carpet with a fluffy speckle
  carpet(d, x, y) {
    d(0, 0, 16, 16, "#8f7fb8");
    for (let i = 0; i < 6; i++) d((x * 7 + i * 5) % 15, (y * 3 + i * 7) % 15, 1, 1, i % 2 ? "#a596cc" : "#7d6ea6");
  },
  // 書斎: dark walnut parquet (herringbone-ish)
  darkwood(d, x, y) {
    d(0, 0, 16, 16, "#5a3b24");
    for (let i = 0; i < 4; i++) d(i * 4, (x + y + i) % 2 ? 0 : 8, 1, 8, "#3e2716");
    d(0, 7, 16, 1, "#3e2716"); d(0, 15, 16, 1, "#3e2716");
  },
  // 趣味部屋: green tatami-like mats
  mat(d, x, y) {
    d(0, 0, 16, 16, "#a7b86a");
    for (let i = 1; i < 16; i += 2) d(0, i, 16, 1, "#98a95e");
    if ((x + y) % 2) d(0, 0, 1, 16, "#5c4a2a"); else d(0, 0, 16, 1, "#5c4a2a");
  },
  // ゲストルーム: pale blue carpet with a diamond pattern
  bluecarpet(d, x, y) {
    d(0, 0, 16, 16, "#9cc0d8");
    d(7, 2, 2, 2, "#b8d6ea"); d(5, 4, 6, 2, "#b8d6ea"); d(7, 6, 2, 2, "#b8d6ea");
  },
};

function drawFloor(room: Room) {
  ctx.fillStyle = "#1b1d23"; ctx.fillRect(0, 0, room.width * T, room.height * T);
  for (const z of room.zones || []) {
    const paint = FLOORS[z.floor] || FLOORS.wood;
    for (let y = z.rect.y; y < z.rect.y + z.rect.h; y++)
      for (let x = z.rect.x; x < z.rect.x + z.rect.w; x++) paint(dotPen(x * T, y * T), x, y, z);
  }
}

function isDoor(room: Room, x: number, y: number) { return (room.doors || []).some(d => d.x === x && d.y === y); }
function isWall(room: Room, x: number, y: number) {
  if (isDoor(room, x, y)) return false;
  return (room.walls || []).some(w => x >= w.x && y >= w.y && x < w.x + w.w && y < w.y + w.h);
}

// Walls: cream plaster with a dark top edge; a shaded base where floor meets the wall.
function drawWalls(room: Room) {
  for (let y = 0; y < room.height; y++) for (let x = 0; x < room.width; x++) {
    const d = dotPen(x * T, y * T);
    if (isDoor(room, x, y)) {
      const vertical = isWall(room, x, y - 1) || isWall(room, x, y + 1);
      d(0, 0, 16, 16, "#7a5430");                       // threshold
      if (vertical) { d(0, 0, 16, 2, "#4a3020"); d(0, 14, 16, 2, "#4a3020"); d(1, 2, 2, 12, "#c9a77a"); }
      else { d(0, 0, 2, 16, "#4a3020"); d(14, 0, 2, 16, "#4a3020"); d(2, 1, 12, 2, "#c9a77a"); }
      continue;
    }
    if (!isWall(room, x, y)) continue;
    d(0, 0, 16, 16, "#e9dcc4");
    d(0, 0, 16, 3, "#3a3027");
    if (!isWall(room, x, y + 1) && y + 1 < room.height) d(0, 13, 16, 3, "#b8a68a");
    if (!isWall(room, x - 1, y) && x > 0) d(0, 3, 1, 13, "#c8b89c");
    if (!isWall(room, x + 1, y) && x + 1 < room.width) d(15, 3, 1, 13, "#c8b89c");
  }
}

// Zones the viewer may not see are closed with the house's curtain (town.js).
function drawHiddenZones(room: Room) {
  for (const id of room.hidden_zones || []) {
    const z = (room.zones || []).find(z => z.id === id);
    if (z) drawCurtain(z);
  }
}
