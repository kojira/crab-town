// generated from web-src/furniture.ts by scripts/build-web.mjs; edit the .ts file, not this one
function dotPen(x0, y0) {
  return (x, y, w, h, c) => {
    ctx.fillStyle = c;
    ctx.fillRect(x0 + x * P, y0 + y * P, w * P, h * P);
  };
}
const WOOD = "#8a5a32", WOOD_D = "#5e3b1f", WOOD_L = "#b07a48";
const FURNITURE = {
  // 窓 (4x1, set into the top wall): wooden frame, panes of sky with clouds, curtains
  window(d, W) {
    d(0, 0, W, 16, WOOD_D);
    d(2, 2, W - 4, 11, "#5aa8e8");
    d(2, 2, W - 4, 4, "#3f8fd6");
    d(8, 6, 8, 2, "#fff");
    d(6, 7, 12, 2, "#f2f8ff");
    d(36, 4, 10, 2, "#fff");
    d(34, 5, 14, 2, "#f2f8ff");
    d(2, 10, W - 4, 3, "#9fd4ff");
    for (let x = 2; x < W - 2; x += 15) d(x + 13, 2, 2, 11, WOOD);
    d(2, 7, W - 4, 1, WOOD);
    d(0, 0, 4, 15, "#d0506a");
    d(W - 4, 0, 4, 15, "#d0506a");
    d(1, 0, 1, 15, "#a83a52");
    d(W - 3, 0, 1, 15, "#a83a52");
    d(0, 13, W, 3, WOOD_L);
  },
  // ソファ (4x2, seen from above, back towards the bottom)
  sofa(d, W, H) {
    const c = "#3f6fb0", cd = "#2b4f85", cl = "#5d8fd4";
    d(1, 2, W - 2, H - 3, cd);
    d(1, H - 9, W - 2, 8, c);
    d(1, H - 9, W - 2, 1, cl);
    d(1, 2, 6, H - 3, c);
    d(W - 7, 2, 6, H - 3, c);
    d(2, 2, 4, 1, cl);
    d(W - 6, 2, 4, 1, cl);
    const seatW = (W - 14) / 3;
    for (let i = 0; i < 3; i++) {
      d(7 + i * seatW + 1, 3, seatW - 2, H - 13, cl);
      d(7 + i * seatW + 1, H - 11, seatW - 2, 1, cd);
    }
    d(W / 2 - 14, H - 12, 8, 6, "#ffd84a");
    d(W / 2 + 6, H - 12, 8, 6, "#f07a9a");
    d(1, H - 1, W - 2, 1, "rgba(0,0,0,0.35)");
  },
  // ベッド (2x3): headboard, two pillows, folded sheet, checked duvet
  bed(d, W, H) {
    d(0, 0, W, 4, WOOD_D);
    d(1, 1, W - 2, 2, WOOD);
    d(1, 4, W - 2, H - 6, "#f4f0e8");
    d(3, 6, 12, 7, "#fffaf0");
    d(17, 6, 12, 7, "#fffaf0");
    d(3, 12, 12, 1, "#cfc7b8");
    d(17, 12, 12, 1, "#cfc7b8");
    d(1, 16, W - 2, 3, "#fff");
    d(1, 19, W - 2, H - 22, "#8e5cc0");
    for (let y = 21; y < H - 4; y += 6) for (let x = 3; x < W - 3; x += 6) d(x, y, 3, 3, "#a97ad8");
    d(1, H - 3, W - 2, 3, WOOD_D);
  },
  // 本棚 (3x2): front view, 3 shelves of coloured book spines
  bookshelf(d, W, H) {
    d(0, 0, W, H, WOOD_D);
    d(1, 1, W - 2, 1, WOOD_L);
    const cols = ["#c0392b", "#2e86c1", "#27ae60", "#f1c40f", "#8e44ad", "#e67e22", "#ecf0f1", "#16a085"];
    for (let s = 0; s < 3; s++) {
      const y = 3 + s * 9;
      d(2, y, W - 4, 7, "#3b2412");
      let x = 3, i = s * 3;
      while (x < W - 5) {
        const bw = 2 + (i % 3 === 0 ? 1 : 0), bh = 5 + i % 2;
        d(x, y + 7 - bh, bw, bh, cols[i % cols.length]);
        if (i % 4 === 0) d(x, y + 7 - bh + 1, bw, 1, "#00000044");
        x += bw + (i % 5 === 4 ? 2 : 0);
        i++;
      }
      d(1, y + 7, W - 2, 2, WOOD);
    }
  },
  // PC (2x1): desk with a monitor showing code, keyboard and mouse
  pc(d, W) {
    d(0, 9, W, 7, WOOD);
    d(0, 9, W, 1, WOOD_L);
    d(0, 15, W, 1, WOOD_D);
    d(4, 0, 18, 9, "#22252e");
    d(5, 1, 16, 6, "#0f2a1a");
    d(6, 2, 7, 1, "#9be37b");
    d(8, 3, 9, 1, "#9be37b");
    d(8, 4, 5, 1, "#ffd84a");
    d(6, 5, 10, 1, "#9be37b");
    d(12, 7, 2, 2, "#8b93a6");
    d(5, 11, 14, 3, "#4a5060");
    for (let x = 6; x < 18; x += 2) d(x, 12, 1, 1, "#d9dde6");
    d(22, 11, 3, 3, "#d9dde6");
    d(26, 2, 4, 7, "#3a3f4d");
    d(27, 3, 2, 1, "#5ce1ff");
  },
  shoebox(d, W, H) {
    d(0, 0, W, H, WOOD);
    d(0, 0, W, 1, WOOD_L);
    for (let y = 2; y < H; y += 16) {
      d(1, y, W - 2, 13, WOOD_D);
      d(W - 4, y + 6, 2, 2, "#e0c070");
    }
    d(3, H - 6, 4, 3, "#c0392b");
    d(8, H - 6, 4, 3, "#2c3e50");
  },
  // キッチン台 (4x1): counter top with sink and faucet, cutting board, cabinets
  counter(d, W) {
    d(0, 0, W, 16, "#d8d8d0");
    d(0, 0, W, 2, "#f2f2ea");
    d(6, 3, 18, 9, "#9aa4ad");
    d(7, 4, 16, 7, "#6f7a84");
    d(14, 1, 2, 4, "#c8ced4");
    d(32, 4, 14, 8, "#e0b070");
    d(36, 7, 6, 2, "#c0392b");
    d(0, 13, W, 3, "#7d6a55");
    for (let x = 0; x < W; x += 16) d(x + 7, 14, 2, 1, "#e0c070");
  },
  // コンロ (2x1): black hob with four burners, two glowing
  stove(d, W) {
    d(0, 0, W, 16, "#2a2a2e");
    d(0, 0, W, 1, "#55555c");
    const ring = (x, y, hot) => {
      d(x, y, 8, 6, hot ? "#e0402a" : "#4a4a52");
      d(x + 2, y + 2, 4, 2, hot ? "#ff9a3c" : "#2a2a2e");
    };
    ring(3, 1, true);
    ring(19, 1, false);
    ring(3, 8, false);
    ring(19, 8, true);
    d(13, 2, 4, 3, "#888");
  },
  // 冷蔵庫 (1x2): tall white box, freezer split, handles
  fridge(d, W, H) {
    d(1, 0, W - 2, H, "#e8eef2");
    d(1, 0, W - 2, 1, "#ffffff");
    d(1, 11, W - 2, 1, "#9aa6ae");
    d(W - 4, 3, 1, 6, "#7d8a92");
    d(W - 4, 14, 1, 12, "#7d8a92");
    d(4, 16, 4, 3, "#ffd84a");
    d(5, 21, 3, 3, "#f07a9a");
    d(1, H - 1, W - 2, 1, "#9aa6ae");
  },
  // ダイニングテーブル (3x2): wooden top, placemats, plates, a vase
  table(d, W, H) {
    d(1, 2, W - 2, H - 4, WOOD);
    d(1, 2, W - 2, 1, WOOD_L);
    d(1, H - 3, W - 2, 1, WOOD_D);
    const plate = (x, y) => {
      d(x, y, 8, 6, "#e8d8b0");
      d(x + 1, y + 1, 6, 4, "#fff");
      d(x + 3, y + 2, 2, 2, "#e6a23c");
    };
    plate(4, 5);
    plate(W - 12, 5);
    plate(4, H - 13);
    plate(W - 12, H - 13);
    d(W / 2 - 2, H / 2 - 4, 4, 6, "#4aa3c8");
    d(W / 2 - 3, H / 2 - 7, 2, 3, "#f07a9a");
    d(W / 2 + 1, H / 2 - 7, 2, 3, "#ffd84a");
  },
  chair(d) {
    d(3, 3, 10, 10, WOOD);
    d(3, 3, 10, 2, WOOD_D);
    d(4, 6, 8, 6, "#c0392b");
  },
  lowtable(d, W) {
    d(1, 3, W - 2, 10, WOOD_L);
    d(1, 12, W - 2, 1, WOOD);
    d(5, 5, 6, 4, "#fff");
    d(6, 6, 4, 2, "#6b3a1f");
    d(17, 5, 10, 6, "#2e86c1");
    d(18, 6, 8, 1, "#fff");
  },
  plant(d) {
    d(4, 10, 8, 6, "#b5603a");
    d(4, 10, 8, 1, "#d27a50");
    d(6, 2, 4, 9, "#2f8f3a");
    d(2, 4, 5, 4, "#3fae4a");
    d(9, 1, 5, 5, "#3fae4a");
    d(3, 0, 4, 3, "#57c862");
    d(10, 6, 4, 3, "#57c862");
  },
  nightstand(d) {
    d(2, 6, 12, 10, WOOD);
    d(2, 11, 12, 1, WOOD_D);
    d(7, 13, 2, 1, "#e0c070");
    d(6, 1, 4, 5, "#ffe9a8");
    d(7, 0, 2, 1, "#fff6d0");
    d(7, 5, 2, 1, "#6b6b6b");
    d(10, 7, 3, 3, "#ff6060");
  },
  wardrobe(d, W) {
    d(0, 0, W, 16, "#c9a77a");
    d(0, 0, W, 1, "#e0c49a");
    d(W / 2, 1, 1, 14, "#8a6a40");
    d(W / 2 - 3, 6, 1, 4, "#5e3b1f");
    d(W / 2 + 3, 6, 1, 4, "#5e3b1f");
  },
  // テレビ + ゲーム機 (3x1): wide TV with a game on screen, console, controller
  tv(d, W) {
    d(0, 11, W, 5, WOOD_D);
    d(4, 0, W - 8, 10, "#111");
    d(5, 1, W - 10, 8, "#1b2a6b");
    d(5, 7, W - 10, 2, "#3aa04a");
    d(12, 4, 3, 3, "#ffd84a");
    d(26, 3, 4, 4, "#e04040");
    d(32, 2, 6, 1, "#fff");
    d(6, 12, 8, 3, "#e8e8f0");
    d(7, 13, 1, 1, "#5ce1ff");
    d(W - 14, 12, 8, 3, "#333");
    d(W - 13, 13, 1, 1, "#e04040");
    d(W - 8, 13, 1, 1, "#4a90e2");
  },
  // アニメ棚 (2x2): glass shelf with colourful figures + a poster above
  figureshelf(d, W, H) {
    d(0, 0, W, 10, "#f4f0ff");
    d(2, 1, 12, 8, "#ff8fc8");
    d(5, 2, 6, 5, "#ffe0f0");
    d(6, 3, 1, 1, "#222");
    d(9, 3, 1, 1, "#222");
    d(18, 1, 12, 8, "#6ad0ff");
    d(22, 2, 4, 5, "#fff");
    d(0, 10, W, H - 10, "#3a3550");
    d(1, 11, W - 2, H - 12, "#5a5478");
    const figs = ["#ff6fa8", "#ffd84a", "#6ad0ff", "#9be37b", "#c08cff", "#ff9a3c"];
    for (let s = 0; s < 2; s++) {
      const y = 12 + s * 10;
      for (let i = 0; i < 5; i++) {
        const x = 3 + i * 6, c = figs[(i + s * 2) % figs.length];
        d(x, y + 2, 3, 5, c);
        d(x, y, 3, 2, "#f2c9a0");
        d(x, y - 1, 3, 1, c);
      }
      d(1, y + 7, W - 2, 1, "#ccc8e0");
    }
  },
  beanbag(d) {
    d(2, 4, 12, 11, "#e67e22");
    d(3, 3, 10, 2, "#f39c50");
    d(5, 6, 6, 4, "#d06a10");
  },
  // 来客用ベッド (2x3): simpler bed with a blue striped duvet
  guestbed(d, W, H) {
    d(0, 0, W, 3, "#c9a77a");
    d(1, 3, W - 2, H - 5, "#f4f0e8");
    d(5, 5, W - 10, 7, "#fff");
    d(1, 15, W - 2, H - 18, "#3a7fc0");
    for (let y = 17; y < H - 3; y += 5) d(1, y, W - 2, 2, "#6aa8e0");
    d(1, H - 3, W - 2, 3, "#c9a77a");
  }
};
function furnitureSize(f) {
  return { w: Math.max(1, f.size && f.size.w || 1), h: Math.max(1, f.size && f.size.h || 1) };
}
function occupies(f, x, y) {
  const s = furnitureSize(f);
  return x >= f.pos.x && y >= f.pos.y && x < f.pos.x + s.w && y < f.pos.y + s.h;
}
function drawFurniture(f) {
  const s = furnitureSize(f), x0 = f.pos.x * T, y0 = f.pos.y * T;
  const paint = FURNITURE[f.kind];
  if (!paint) {
    ctx.fillStyle = "#ccc";
    ctx.fillRect(x0 + 2, y0 + 2, s.w * T - 4, s.h * T - 4);
    return;
  }
  paint(dotPen(x0, y0), s.w * 16, s.h * 16);
}
