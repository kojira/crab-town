// crab-town viewer: shared constants and 16x16 dot sprites (Canvas only, no images).
const T = 32;          // tile size (px)
const P = T / 16;      // sprite dot size: sprites are 16x16 dots
const STEP_MS = 250;   // walking interpolation (matches default CRAB_TICK)
const cv = document.getElementById("c"), ctx = cv.getContext("2d");
const statusEl = document.getElementById("status"), logEl = document.getElementById("log");
let rooms = {}, actors = {};
const view = {}; // per-actor render state: from / to / start / flip / frame


// ---- dot sprites (16x16, "." = transparent) ---------------------------------
const SPRITES = {
  // 窓: 木枠 + 十字の桟 + ガラスに映る空（上ほど濃い青、雲、光の反射）
  window: {
    pal: { b:"#6b4a2b", k:"#4f9fe0", s:"#7ec8ff", l:"#b9e3ff", w:"#ffffff", r:"#e8f6ff", c:"#8a6440" },
    rows: [
      "bbbbbbbbbbbbbbbb",
      "bkkkkkkbbkkkkkkb",
      "bkkkkkkbbkkwwkkb",
      "bswwsssbbswwwwsb",
      "bwwwwssbbssssssb",
      "bssssssbbssssssb",
      "bllllrlbblllllrb",
      "bbbbbbbbbbbbbbbb",
      "bbbbbbbbbbbbbbbb",
      "bsssssrbbssssssb",
      "bssssrsbbssssssb",
      "blllrllbbllllrlb",
      "bllrlllbblllrllb",
      "bbbbbbbbbbbbbbbb",
      ".cccccccccccccc.",
      "................",
    ],
  },
  // PC: モニター（画面にコード）+ スタンド + キーボード
  pc: {
    pal: { k:"#2b2f3a", s:"#10301a", y:"#9be37b", g:"#ffd84a", m:"#8b93a6", n:"#4a5060", w:"#d9dde6" },
    rows: [
      ".kkkkkkkkkkkkkk.",
      ".kssssssssssssk.",
      ".ksyyyyyssssssk.",
      ".kssssyyyyyyssk.",
      ".ksyyyssssssssk.",
      ".kssssyyyyssssk.",
      ".kssssssssssssk.",
      ".kkkkkkkkkkkkkk.",
      ".kkkkkkkgkkkkkk.",
      "......mmmm......",
      "....mmmmmmmm....",
      "................",
      ".nnnnnnnnnnnnnn.",
      ".nwnwnwnwnwnwnn.",
      ".nnnwwwwwwwwnnn.",
      "................",
    ],
  },
  // ベッド（上から見た図）: ヘッドボード + 枕 + 折り返した掛け布団 + フットボード
  bed: {
    pal: { h:"#6b4a2b", H:"#8a6440", f:"#5a3d22", w:"#f4f0e8", O:"#b8b2a6", o:"#fffaf0",
           t:"#f4d3ea", p:"#e39bd2", q:"#b56fa3" },
    rows: [
      ".hhhhhhhhhhhhhh.",
      ".hHHHHHHHHHHHHh.",
      ".fwwwwwwwwwwwwf.",
      ".fwOOOOOOOOOOwf.",
      ".fwOooooooooOwf.",
      ".fwOOOOOOOOOOwf.",
      ".fwwwwwwwwwwwwf.",
      ".fttttttttttttf.",
      ".fppppppppppppf.",
      ".fpqppqppqppqpf.",
      ".fppppppppppppf.",
      ".fpqppqppqppqpf.",
      ".fppppppppppppf.",
      ".hhhhhhhhhhhhhh.",
      ".h............h.",
      "................",
    ],
  },
};

// のすたろう: 17歳・高校デビュー。前髪に黄色メッシュ、両耳ピアス、黒ジャケットに⚡。
const NOSTAROU_PAL = {
  H:"#2a2440", Y:"#ffd84a", S:"#f2c9a0", E:"#111111", M:"#b5605a",
  P:"#dfe6ff", J:"#22242b", Z:"#ffd84a", L:"#3b4a6b", B:"#111111",
};
const NOSTAROU_BODY = [
  "....HHHHHHH.....",
  "...HHHHYHHHH....",
  "..HHHHYYHHHHH...",
  "..HHSSSSSSSHH...",
  "..HSSESSSESSH...",
  "..SSSSSSSSSSS...",
  "..PSSSSMSSSSP...",
  "....JJSSSJJ.....",
  "...JJJJJJJJJ....",
  "..JJJJJJJZJJJ...",
  "..SJJJJJZJJJS...",
  "..SJJJZZZZJJS...",
  "...JJJJZJJJJ....",
];
const NOSTAROU_LEGS = [
  [ "....LLL.LLL.....", "....LLL.LLL.....", "...BBB...BBB...." ], // stand
  [ "....LLL..LLL....", "...LLL....LL....", "...BB.....BBB..." ], // step
];
