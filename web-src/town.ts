// crab-town viewer: the town -- labomi's house (pink gal room), the garden and
// per-house curtains. Same contract as furniture.js: paint(d, W, H) in dots.
const PINK = "#f59ac0", PINK_D = "#d8688f", PINK_L = "#ffd0e2", LILAC = "#c8a8ff";

Object.assign(FLOORS, {
  // らぼみの LDK: whitewashed pink-oak planks
  pinkwood(d, x, y) {
    d(0, 0, 16, 16, "#e9c6b8"); d(0, 7, 16, 1, "#cfa294"); d(0, 15, 16, 1, "#cfa294");
    d(y % 2 ? 5 : 11, 0, 1, 7, "#cfa294"); d(y % 2 ? 13 : 3, 8, 1, 7, "#cfa294"); d(7, 3, 3, 1, "#f6dcd0");
  },
  // らぼみの玄関: white marble with grey veins
  marble(d, x, y) {
    d(0, 0, 16, 16, "#f2eef0"); d(0, 15, 16, 1, "#cdc6ca"); d(15, 0, 1, 16, "#cdc6ca");
    if ((x + y) % 2) { d(2, 4, 5, 1, "#d6d0d4"); d(6, 5, 4, 1, "#d6d0d4"); }
    else { d(8, 9, 5, 1, "#ddd6da"); d(4, 11, 4, 1, "#ddd6da"); }
  },
  // らぼみの部屋: fluffy pink carpet
  pinkcarpet(d, x, y) {
    d(0, 0, 16, 16, "#f3b3cc");
    for (let i = 0; i < 6; i++) d((x * 7 + i * 5) % 15, (y * 3 + i * 7) % 15, 1, 1, i % 2 ? "#ffd0e2" : "#e398b6");
  },
  // 庭: grass with tufts
  grass(d, x, y) {
    d(0, 0, 16, 16, (x + y) % 2 ? "#6cb85a" : "#65b053");
    for (let i = 0; i < 4; i++) {
      const gx = (x * 5 + i * 7) % 14, gy = (y * 7 + i * 5) % 13;
      d(gx, gy + 1, 1, 2, "#4f9a40"); d(gx + 1, gy, 1, 3, "#86cc6e");
    }
  },
});

// Small heart, 5x4 dots.
function heart(d, x, y, c) { d(x, y, 2, 1, c); d(x + 3, y, 2, 1, c); d(x, y + 1, 5, 1, c); d(x + 1, y + 2, 3, 1, c); d(x + 2, y + 3, 1, 1, c); }

Object.assign(FURNITURE, {
  // ピンクのソファ (4x2): same build as the sofa, pink with heart cushions
  pinksofa(d, W, H) {
    d(1, 2, W - 2, H - 3, PINK_D);
    d(1, H - 9, W - 2, 8, PINK); d(1, H - 9, W - 2, 1, PINK_L);
    d(1, 2, 6, H - 3, PINK); d(W - 7, 2, 6, H - 3, PINK);
    const seatW = (W - 14) / 2;
    for (let i = 0; i < 2; i++) { d(7 + i * seatW + 1, 3, seatW - 2, H - 13, PINK_L); d(7 + i * seatW + 1, H - 11, seatW - 2, 1, PINK_D); }
    heart(d, W / 2 - 15, H - 14, "#ff4f8b"); heart(d, W / 2 + 10, H - 14, "#ffffff");
    d(1, H - 1, W - 2, 1, "rgba(0,0,0,0.35)");
  },
  // ハートのラグ (walkable): a big heart on a cream mat
  heartrug(d, W, H) {
    d(1, 1, W - 2, H - 2, "#fff0f5"); d(2, 2, W - 4, H - 4, "#ffe0ec");
    const s = Math.max(1, Math.floor(Math.min(W, H) / 9)), cx = Math.floor(W / 2), cy = Math.floor(H / 2);
    const rows = [[-3, -1, 2, 1, 2], [-2, -4, 8], [-1, -4, 8], [0, -3, 6], [1, -2, 4], [2, -1, 2]];
    for (const r of rows) {
      if (r.length === 5) { d(cx + (-4) * s, cy + r[0] * s, 3 * s, s, "#ff7aa8"); d(cx + s, cy + r[0] * s, 3 * s, s, "#ff7aa8"); continue; }
      d(cx + r[1] * s, cy + r[0] * s, r[2] * s, s, "#ff7aa8");
    }
    d(cx - 3 * s, cy - 2 * s, s, s, "#ffc0d8");
  },
  // ベッド (2x3): white frame, pink duvet with hearts, a bear on the pillow
  pinkbed(d, W, H) {
    d(0, 0, W, 4, "#fff4f8"); d(1, 1, W - 2, 2, PINK_L);
    d(1, 4, W - 2, H - 6, "#fffafc");
    d(3, 6, 12, 7, "#ffffff"); d(17, 6, 12, 7, "#ffffff"); d(3, 12, 26, 1, "#e8d8e0");
    d(20, 7, 6, 5, "#c89870"); d(19, 6, 2, 2, "#a87850"); d(25, 6, 2, 2, "#a87850"); d(22, 9, 2, 1, "#5e3b1f"); // bear
    d(1, 16, W - 2, 3, "#fff");
    d(1, 19, W - 2, H - 22, PINK);
    for (let y = 22; y < H - 6; y += 8) for (let x = 4; x < W - 6; x += 10) heart(d, x + (y % 16 ? 4 : 0), y, "#ffffff");
    d(1, H - 3, W - 2, 3, "#fff4f8");
  },
  // ドレッサー (2x1): mirror with bulbs, cosmetics on the top
  dresser(d, W) {
    d(0, 9, W, 7, "#fff4f8"); d(0, 9, W, 1, "#ffffff"); d(0, 15, W, 1, "#e0c8d4");
    d(8, 0, 16, 9, "#e8d0dc"); d(10, 1, 12, 7, "#bfe6f5"); d(11, 2, 3, 3, "#e8f8ff");
    for (let i = 0; i < 3; i++) { d(7, 1 + i * 3, 1, 1, "#ffe9a8"); d(24, 1 + i * 3, 1, 1, "#ffe9a8"); }
    d(3, 11, 2, 3, "#ff4f8b"); d(6, 11, 3, 3, LILAC); d(23, 11, 2, 3, "#ffd84a"); d(26, 12, 3, 2, "#f2c9a0");
    d(W / 2 - 1, 13, 2, 1, "#e0c070");
  },
  // ぬいぐるみ: pink bunny
  plush(d) {
    d(4, 0, 2, 6, "#ffc0d8"); d(10, 0, 2, 6, "#ffc0d8"); d(5, 1, 1, 4, PINK_D); d(10, 1, 1, 4, PINK_D);
    d(3, 5, 10, 6, "#ffc0d8"); d(5, 7, 1, 1, "#222"); d(10, 7, 1, 1, "#222"); d(7, 8, 2, 1, PINK_D);
    d(4, 11, 8, 4, "#ffc0d8"); d(3, 12, 2, 2, "#ffc0d8"); d(11, 12, 2, 2, "#ffc0d8");
  },
  // 自撮りライト: ring light on a tripod, phone in the middle
  ringlight(d) {
    d(1, 0, 14, 10, "rgba(255,250,230,0.25)");
    d(3, 1, 10, 1, "#fff"); d(3, 8, 10, 1, "#fff"); d(2, 2, 1, 6, "#fff"); d(13, 2, 1, 6, "#fff");
    d(6, 3, 4, 4, "#333"); d(7, 4, 2, 2, "#5ce1ff");
    d(7, 9, 2, 4, "#555"); d(4, 13, 2, 3, "#555"); d(10, 13, 2, 3, "#555");
  },
  // 布団 (2x3): futon laid on tatami, striped quilt
  futon(d, W, H) {
    d(1, 1, W - 2, H - 2, "#f4f0e8"); d(1, H - 2, W - 2, 1, "#cfc7b8");
    d(5, 3, W - 10, 7, "#fff");
    d(1, 13, W - 2, H - 15, "#e88a5a");
    for (let y = 16; y < H - 3; y += 6) d(1, y, W - 2, 2, "#ffd0a0");
  },
  // ノートPC (2x1): white desk with a pink laptop and a mug
  laptop(d, W) {
    d(0, 9, W, 7, "#fff4f8"); d(0, 9, W, 1, "#ffffff"); d(0, 15, W, 1, "#e0c8d4");
    d(5, 1, 16, 9, PINK); d(6, 2, 14, 6, "#2a1a30");
    d(7, 3, 6, 1, "#ff9ac8"); d(8, 4, 8, 1, "#c8a8ff"); d(7, 5, 5, 1, "#9be37b");
    d(4, 10, 18, 3, PINK_D); for (let x = 6; x < 20; x += 2) d(x, 11, 1, 1, "#ffe0ec");
    d(24, 10, 5, 5, "#fff"); d(25, 11, 3, 2, "#b5603a"); heart(d, 25, 3, "#ff4f8b");
  },
  // 飛び石: flat grey stone on the grass (walkable)
  steppingstone(d) {
    d(3, 4, 10, 8, "#9a9a94"); d(2, 5, 12, 6, "#9a9a94"); d(4, 3, 8, 1, "#aeaea8");
    d(4, 5, 4, 2, "#b6b6b0"); d(3, 11, 10, 1, "#6e6e68"); d(9, 8, 2, 1, "#86867f");
  },
  // 木: round crown with a trunk and shadow
  tree(d) {
    d(3, 13, 10, 3, "rgba(0,0,0,0.25)"); d(7, 10, 2, 5, "#7a4a28");
    d(2, 3, 12, 8, "#2f8f3a"); d(4, 1, 8, 2, "#2f8f3a"); d(3, 2, 6, 4, "#3fae4a"); d(9, 5, 4, 4, "#3fae4a");
    d(4, 3, 3, 2, "#57c862"); d(5, 8, 2, 1, "#e04040"); d(10, 4, 1, 1, "#e04040");
  },
  // 花壇: brick bed with flowers
  flower(d) {
    d(1, 10, 14, 5, "#b5603a"); d(1, 10, 14, 1, "#d27a50"); d(2, 8, 12, 3, "#5a3a20");
    const fl = ["#ff6fa8", "#ffd84a", "#ffffff", "#c08cff"];
    for (let i = 0; i < 4; i++) { const x = 2 + i * 3; d(x + 1, 4, 1, 5, "#3fae4a"); d(x, 2, 3, 3, fl[i]); d(x + 1, 3, 1, 1, "#e67e22"); }
  },
  // 郵便受け: post with a red box
  postbox(d) {
    d(7, 9, 2, 7, "#5e3b1f"); d(3, 2, 10, 8, "#d04040"); d(3, 2, 10, 2, "#e86060");
    d(5, 5, 6, 1, "#222"); d(12, 3, 1, 3, "#ffd84a");
  },
});

// ---- らぼみ: gal, long pink hair with a bow, gold hoop earrings, lilac hoodie ----
const LABOMI_PAL = {
  H:"#ff8fbf", R:"#ff4f8b", S:"#f8d6b8", E:"#3a2040", W:"#ffffff", M:"#e0607a",
  G:"#ffd84a", J:"#c8a8ff", K:"#f4f0ff", B:"#ff4f8b",
};
const LABOMI_BODY = [
  "...HHHHHHHRR....",
  "..HHHHHHHHRRR...",
  "..HHHHHHHHHHH...",
  ".HHHSSSSSSSHHH..",
  ".HHSWESSSEWSHH..",
  ".HHSSSSSSSSSHH..",
  ".HGSSSSMSSSSGH..",
  ".HH..SSSSS..HH..",
  ".HHJJJJJJJJJHH..",
  ".HJJJJKKKJJJJH..",
  "..SJJJJKJJJJS...",
  "..SJJJJJJJJJS...",
  "...KKKKKKKKK....",
];
const LABOMI_LEGS = [
  [ "....SSS.SSS.....", "....SSS.SSS.....", "...BBB...BBB...." ],
  [ "....SSS..SSS....", "...SSS....SS....", "...BB.....BBB..." ],
];
const SPRITES = {
  labomi: { body: LABOMI_BODY, legs: LABOMI_LEGS, pal: LABOMI_PAL },
  nostarou: { body: NOSTAROU_BODY, legs: NOSTAROU_LEGS, pal: NOSTAROU_PAL },
};
function spriteFor(a) { return SPRITES[a.id] || SPRITES.nostarou; }

// ---- per-house curtains (暗幕) ----------------------------------------------
// A zone the viewer may not see is covered by a drawn curtain in its house's
// colour (labomi: rose velvet, nostarou: navy), so whose room is closed shows
// without a label. Opaque: nothing inside shows through.
const CURTAINS = {
  "labomi-house": { base: "#7a2f52", fold: "#5e2240", hi: "#a04870", rod: "#e0c070" },
  "nostarou-house": { base: "#2c3560", fold: "#1e2546", hi: "#45508a", rod: "#ffd84a" },
};
const CURTAIN_DEFAULT = { base: "#33333d", fold: "#24242c", hi: "#4a4a56", rod: "#c8b060" };
function drawCurtain(z) {
  const c = CURTAINS[z.house] || CURTAIN_DEFAULT;
  const x0 = z.rect.x * T, y0 = z.rect.y * T, w = z.rect.w * T, h = z.rect.h * T;
  ctx.fillStyle = c.base; ctx.fillRect(x0, y0, w, h);
  for (let x = 0; x < w; x += 8 * P) {                  // vertical folds
    ctx.fillStyle = c.fold; ctx.fillRect(x0 + x, y0, 2 * P, h);
    ctx.fillStyle = c.hi; ctx.fillRect(x0 + x + 4 * P, y0, P, h);
  }
  ctx.fillStyle = c.fold;                                 // scalloped hem
  for (let x = 0; x < w; x += 8 * P) ctx.fillRect(x0 + x + 2 * P, y0 + h - 2 * P, 4 * P, 2 * P);
  ctx.fillStyle = c.rod; ctx.fillRect(x0, y0, w, 2 * P);  // rod + rings
  for (let x = 2 * P; x < w; x += 8 * P) ctx.fillRect(x0 + x, y0 + 2 * P, 2 * P, P);
  const p = dotPen(x0 + w / 2 - 8 * P, y0 + h / 2 - 8 * P); // padlock
  p(4, 6, 8, 7, c.rod); p(5, 2, 1, 4, c.rod); p(10, 2, 1, 4, c.rod); p(5, 1, 6, 1, c.rod); p(7, 8, 2, 3, "#333");
}
