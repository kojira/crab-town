// crab-town viewer: shared constants and the character sprite (Canvas only, no images).
// Furniture dot art lives in furniture.js.
const T = 32;          // tile size (px)
const P = T / 16;      // sprite dot size: sprites are 16x16 dots
const STEP_MS = 250;   // walking interpolation (matches default CRAB_TICK)
const cv = document.getElementById("c"), ctx = cv.getContext("2d");
const statusEl = document.getElementById("status"), logEl = document.getElementById("log");
let rooms = {}, actors = {};
const view = {}; // per-actor render state: from / to / start / flip / frame


// ---- character sprite (16x16, "." = transparent) ----------------------------
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
