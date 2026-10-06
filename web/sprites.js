// generated from web-src/sprites.ts by scripts/build-web.mjs; edit the .ts file, not this one
const T = 32;
const P = T / 16;
const STEP_MS = 250;
const cv = document.getElementById("c"), ctx = cv.getContext("2d");
const statusEl = document.getElementById("status"), logEl = document.getElementById("log");
let rooms = {}, actors = {};
const view = {};
const NOSTAROU_PAL = {
  H: "#2a2440",
  Y: "#ffd84a",
  S: "#f2c9a0",
  E: "#111111",
  M: "#b5605a",
  P: "#dfe6ff",
  J: "#22242b",
  Z: "#ffd84a",
  L: "#3b4a6b",
  B: "#111111"
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
  "...JJJJZJJJJ...."
];
const NOSTAROU_LEGS = [
  ["....LLL.LLL.....", "....LLL.LLL.....", "...BBB...BBB...."],
  // stand
  ["....LLL..LLL....", "...LLL....LL....", "...BB.....BBB..."]
  // step
];
