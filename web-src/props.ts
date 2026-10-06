// crab-town viewer: small props (1x1 mostly) and the water area, as dot art.
// Same contract as furniture.js: paint(d, W, H) with d(x, y, w, h, color) in dots.
const WHITE = "#f4f6f8", PORC = "#e6ecf0", PORC_D = "#a9b6c0", CHROME = "#c8d0d8";

Object.assign(FURNITURE, {
  // 玄関マット: striped coir mat
  doormat(d, W, H) {
    d(1, 3, W - 2, H - 6, "#a0703a"); d(2, 4, W - 4, H - 8, "#c08a48");
    for (let y = 5; y < H - 4; y += 2) d(3, y, W - 6, 1, "#a0703a");
  },
  // 傘立て: blue cylinder with two umbrella handles
  umbrella(d) {
    d(4, 6, 8, 9, "#3a6ea8"); d(4, 6, 8, 1, "#6a9ed8"); d(5, 14, 6, 1, "#24466e");
    d(6, 1, 1, 6, "#c0392b"); d(5, 0, 2, 2, "#c0392b"); d(9, 2, 1, 5, "#2c3e50"); d(9, 1, 2, 2, "#2c3e50");
  },
  // コート掛け: pole on a round foot, coat and hat hanging
  coatrack(d) {
    d(4, 14, 8, 2, WOOD_D); d(7, 2, 2, 13, WOOD);
    d(3, 4, 4, 8, "#7a4a8a"); d(3, 4, 4, 1, "#9a6aaa");   // coat
    d(9, 1, 5, 3, "#2c3e50"); d(10, 0, 3, 1, "#2c3e50");  // hat
  },
  // キッチンマット: long checked mat
  kitchenmat(d, W, H) {
    d(0, 3, W, H - 6, "#e8e0c8");
    for (let x = 0; x < W; x += 4) for (let y = 3; y < H - 3; y += 4) if (((x + y) / 4) % 2 < 1) d(x, y, 4, 4, "#d06a5a");
    d(0, 3, W, 1, "#b85a4a"); d(0, H - 4, W, 1, "#b85a4a");
  },
  // ゴミ箱: grey pedal bin, crumpled paper peeking out
  trash(d) {
    d(4, 5, 8, 10, "#7d8a92"); d(3, 4, 10, 2, "#9aa6ae"); d(4, 15, 8, 1, "#4a5560");
    d(5, 7, 1, 7, "#6a767e"); d(10, 7, 1, 7, "#6a767e");
    d(6, 2, 3, 3, "#f4f4ec"); d(8, 3, 3, 2, "#dcdcd0");
  },
  // 掛け時計 (on the wall): round face with hands
  clock(d) {
    d(4, 3, 8, 10, "#5e3b1f"); d(3, 4, 10, 8, "#5e3b1f");
    d(5, 4, 6, 8, "#fffaf0"); d(4, 5, 8, 6, "#fffaf0");
    d(8, 5, 1, 4, "#222"); d(8, 8, 3, 1, "#c0392b");
    d(8, 4, 1, 1, "#444"); d(8, 11, 1, 1, "#444"); d(4, 8, 1, 1, "#444"); d(11, 8, 1, 1, "#444");
  },
  // 食器棚: glass doors with plates and cups
  sideboard(d, W) {
    d(0, 0, W, 16, WOOD_D); d(1, 1, W - 2, 8, "#bfe0ea");
    for (let x = 3; x < W - 3; x += 6) { d(x, 3, 4, 4, "#fff"); d(x + 1, 4, 2, 2, "#4aa3c8"); }
    d(1, 10, W - 2, 5, WOOD); d(W / 2, 10, 1, 5, WOOD_D); d(W / 2 - 3, 12, 2, 1, "#e0c070"); d(W / 2 + 2, 12, 2, 1, "#e0c070");
  },
  // フロアランプ: warm shade with a glow, thin pole, round foot
  floorlamp(d) {
    d(1, 0, 14, 8, "rgba(255,230,140,0.25)");
    d(4, 1, 8, 5, "#ffe9a8"); d(3, 5, 10, 1, "#e8c870"); d(5, 2, 6, 1, "#fff6d0");
    d(7, 6, 2, 8, "#555"); d(4, 14, 8, 2, "#333");
  },
  // ラグ: woven rug with border and fringe (walkable)
  rug(d, W, H) {
    d(2, 2, W - 4, H - 4, "#4a7a9a"); d(4, 4, W - 8, H - 8, "#e8d8b0"); d(6, 6, W - 12, H - 12, "#6a9aba");
    for (let x = 8; x < W - 8; x += 8) d(x, H / 2 - 1, 4, 2, "#e8d8b0");
    for (let y = 3; y < H - 3; y += 3) { d(0, y, 2, 1, "#e8d8b0"); d(W - 2, y, 2, 1, "#e8d8b0"); }
  },
  // デスクチェア: round seat on a 5-star base, back toward the bottom
  deskchair(d) {
    d(2, 13, 12, 2, "#333"); d(7, 10, 2, 4, "#555");
    d(3, 2, 10, 8, "#2c3e50"); d(4, 3, 8, 6, "#3e5871");
    d(3, 9, 10, 3, "#1e2a36");
  },
  // 便器: white bowl with an open seat, tank against the wall
  toilet(d) {
    d(3, 0, 10, 4, PORC); d(3, 3, 10, 1, PORC_D); d(11, 1, 2, 1, CHROME);
    d(4, 4, 8, 10, WHITE); d(3, 6, 10, 6, WHITE);
    d(5, 6, 6, 6, "#9fd4e8"); d(6, 7, 4, 4, "#7ac0dc");
    d(4, 14, 8, 1, PORC_D);
  },
  // トイレットペーパー: holder on the wall, roll with a hanging sheet
  paper(d) {
    d(3, 2, 10, 2, CHROME); d(4, 4, 8, 6, WHITE); d(4, 4, 8, 1, "#ddd");
    d(5, 10, 6, 4, WHITE); d(5, 13, 6, 1, "#ccc");
    d(7, 6, 2, 2, "#bbb"); // core
  },
  // トイレマット: fluffy pastel mat
  toiletmat(d) {
    d(2, 3, 12, 11, "#f0a8c0"); d(3, 2, 10, 13, "#f0a8c0");
    for (let i = 0; i < 6; i++) d(3 + (i * 5) % 10, 4 + (i * 3) % 9, 1, 1, "#f8c8d8");
  },
  // 洗面台: basin with faucet, mirror strip, cup and toothbrush
  vanity(d, W) {
    d(0, 0, W, 4, "#bfe0ea"); d(1, 1, W - 2, 2, "#e0f4fa");  // mirror
    d(0, 4, W, 12, WHITE); d(0, 15, W, 1, PORC_D);
    if (W < 32) { // 1-tile vanity: basin only, everything stays inside the tile
      d(2, 6, 12, 7, PORC); d(3, 7, 10, 5, "#b8dce8"); d(7, 4, 2, 4, CHROME); d(7, 9, 2, 1, "#555");
      return;
    }
    d(6, 6, 16, 7, PORC); d(8, 7, 12, 5, "#b8dce8"); d(13, 4, 2, 4, CHROME); d(13, 9, 2, 1, "#555");
    d(25, 7, 4, 5, "#4aa3c8"); d(26, 4, 1, 4, "#e04040"); d(27, 4, 1, 4, "#40c070"); // cup, brushes
  },
  // 洗濯機: front loader with a round glass door
  washer(d) {
    d(1, 0, 14, 16, WHITE); d(1, 0, 14, 3, "#dde3e8"); d(2, 1, 3, 1, "#40c070"); d(10, 1, 3, 1, "#555");
    d(4, 5, 8, 9, "#8a96a0"); d(3, 6, 10, 7, "#8a96a0");
    d(5, 6, 6, 7, "#5aa8e8"); d(4, 7, 8, 5, "#5aa8e8"); d(6, 8, 3, 2, "#cfe8ff");
  },
  // 洗濯カゴ: wicker basket with clothes
  laundry(d) {
    d(2, 5, 12, 10, "#c9a060"); for (let y = 6; y < 15; y += 2) d(2, y, 12, 1, "#a07a40");
    d(3, 3, 5, 3, "#e05a6a"); d(7, 2, 5, 4, "#5a8ad8"); d(10, 4, 3, 2, "#fff");
  },
  // シャワー: wall head, hose, drain on tiled floor
  shower(d) {
    d(10, 0, 4, 3, CHROME); d(11, 3, 2, 1, "#9aa"); d(12, 3, 1, 8, "#9aa"); d(11, 10, 3, 2, CHROME);
    for (let i = 0; i < 5; i++) d(10 + (i % 3), 4 + i * 2, 1, 1, "#7ac8f0"); // droplets
    d(3, 11, 4, 3, "#7a8a96"); d(4, 12, 2, 1, "#4a5660");                 // drain
    d(2, 2, 3, 4, "#f0c040"); // shampoo bottle
  },
  // 浴槽 (2x3): long tub with water and a rubber duck
  bathtub(d, W, H) {
    d(0, 0, W, H, PORC); d(0, 0, W, 1, WHITE); d(0, H - 1, W, 1, PORC_D);
    d(3, 3, W - 6, H - 6, "#5ab0e0"); d(4, 4, W - 8, H - 8, "#7ac8f0");
    for (let y = 8; y < H - 6; y += 7) d(6, y, 10, 1, "#a8e0ff");
    d(W - 12, 12, 6, 4, "#ffd84a"); d(W - 10, 10, 4, 3, "#ffd84a"); d(W - 7, 11, 2, 1, "#e67e22"); // duck
    d(W / 2 - 2, 1, 4, 2, CHROME); // faucet
  },
  // バスマット: absorbent mat
  bathmat(d) {
    d(1, 2, 14, 12, "#7ac0a8"); d(2, 3, 12, 10, "#9ad8c0");
    for (let x = 3; x < 13; x += 3) d(x, 4, 1, 8, "#7ac0a8");
  },
} satisfies Record<string, FurniturePainter>);

// Floors for the water area.
Object.assign(FLOORS, {
  // トイレ / 浴室: small white tiles with grey grout
  bathtile(d) {
    d(0, 0, 16, 16, "#e4eef2");
    for (let i = 0; i < 16; i += 4) { d(i, 0, 1, 16, "#b8c6cc"); d(0, i, 16, 1, "#b8c6cc"); }
  },
  // 洗面所: cushion floor with a faint pattern
  cushionfloor(d, x, y) {
    d(0, 0, 16, 16, "#e8dcc0");
    d(0, 0, 16, 1, "#d4c6a6"); d(0, 0, 1, 16, "#d4c6a6");
    if ((x + y) % 2) d(6, 6, 4, 4, "#d8c8a4");
  },
} satisfies Record<string, FloorPainter>);

// In-use lamp on the door of an occupied private zone (toilet / bath): red when
// in use, green when free. Only the flag is known to the viewer, never who.
function drawUseLamps(room: Room) {
  const inUse = new Set(room.in_use || []);
  for (const z of room.zones || []) {
    if (!z.private) continue;
    const door = (room.doors || []).find(p => isDoor(room, p.x, p.y) && touches(z.rect, p));
    if (!door) continue;
    const on = inUse.has(z.id), d = dotPen(door.x * T, door.y * T);
    d(5, 4, 6, 8, "#222"); d(6, 5, 4, 6, on ? "#ff3b3b" : "#3bd16f");
    if (on) { ctx.fillStyle = "rgba(255,60,60,0.25)"; ctx.fillRect(door.x * T - 4, door.y * T - 4, T + 8, T + 8); }
  }
}
function touches(r: Rect, p: Pos) {
  return [[1, 0], [-1, 0], [0, 1], [0, -1]].some(([dx, dy]) => {
    const x = p.x + dx, y = p.y + dy;
    return x >= r.x && y >= r.y && x < r.x + r.w && y < r.y + r.h;
  });
}
