// crab-town viewer: narrow-screen viewport (fixed-scale map, drag to scroll,
// jump buttons from the layout, minimap). Wide screens keep the whole-town fit.
// Uses the globals of sprites.js (cv, T, rooms, actors) and CrabViewport.
const CrabView = (() => {
  const V = CrabViewport;
  const stage = document.getElementById("stage");
  const mini = document.getElementById("minimap"), mctx = mini.getContext("2d");
  const jumps = document.getElementById("jumps");
  const st = { mode: "fit", tile: T, focused: false, self: null, jumpKey: "", drag: null, justDragged: false, mScale: 2 };

  const room = () => Object.values(rooms)[0];
  const townW = () => (room() ? room().width : cv.width / T);
  const townH = () => (room() ? room().height : cv.height / T);
  const content = () => ({ w: townW() * st.tile, h: townH() * st.tile });
  const scrollPos = () => ({ x: stage.scrollLeft, y: stage.scrollTop });
  function scrollTo(o) {
    const c = content(), v = V.clampView(o.x, o.y, stage.clientWidth, stage.clientHeight, c.w, c.h);
    stage.scrollLeft = Math.max(0, v.x); stage.scrollTop = Math.max(0, v.y);
  }
  function centerOn(tx, ty) {
    const c = content();
    scrollTo(V.centerOn(tx, ty, st.tile, stage.clientWidth, stage.clientHeight, c.w, c.h));
  }

  // Pick fit / scroll from the space the page has, then size the stage.
  function layout() {
    const availW = document.documentElement.clientWidth - 16;
    const mode = V.chooseMode(availW, townW());
    if (mode !== st.mode) { st.mode = mode; document.body.classList.toggle("narrow", mode === "scroll"); }
    if (mode === "fit") { cv.style.width = cv.style.height = stage.style.height = ""; window.LABEL_SCALE = 1; return; }
    const chat = document.getElementById("chat"), chatH = chat ? chat.offsetHeight : 0;
    document.body.style.setProperty("--chat-h", chatH + "px");
    const top = stage.getBoundingClientRect().top + window.scrollY;
    // the status / relay lines sit between the map and the chat panel
    const below = ["status", "relays"].reduce((s, id) => { const e = document.getElementById(id); return s + (e ? e.offsetHeight + 4 : 0); }, 4);
    const stageH = V.stageHeight(window.innerHeight, top, below, chatH);
    const keep = scrollPos(), old = st.tile;
    st.tile = V.scrollTile(stageH, townH());
    window.LABEL_SCALE = V.labelScale(st.tile);
    cv.style.width = townW() * st.tile + "px";
    cv.style.height = townH() * st.tile + "px";
    stage.style.height = stageH + "px";
    const c = content();
    st.mScale = V.minimapScale(townW(), townH(), Math.min(140, stage.clientWidth * 0.4), 60);
    const mw = townW() * st.mScale, mh = townH() * st.mScale; // assigning clears it: only on change
    if (mini.width !== mw) mini.width = mw;
    if (mini.height !== mh) mini.height = mh;
    if (old !== st.tile) { // keep the same tile in the middle of the view
      const k = st.tile / old, hw = stage.clientWidth / 2, hh = stage.clientHeight / 2;
      scrollTo({ x: (keep.x + hw) * k - hw, y: (keep.y + hh) * k - hh });
    }
    if (c.w <= stage.clientWidth) scrollTo({ x: 0, y: 0 });
  }

  // First snapshot: look at the viewer (or the garden); rebuild jump buttons when the layout/self changes.
  function onWorld() {
    const r = room();
    if (!r) return;
    layout();
    if (!st.focused && st.mode === "scroll") {
      const f = V.initialFocus(r, actors, st.self);
      if (f) { centerOn(f.x, f.y); st.focused = true; }
    }
    const ts = V.jumpTargets(r, actors, st.self);
    const key = ts.map(t => t.id + t.label + t.x + "," + t.y).join("|");
    if (key === st.jumpKey) return;
    st.jumpKey = key;
    jumps.replaceChildren(...ts.map(t => {
      const b = document.createElement("button");
      b.type = "button"; b.textContent = t.label; b.dataset.target = t.id;
      b.onclick = () => centerOn(t.x, t.y);
      return b;
    }));
  }
  function setSelf(id) { st.self = id || null; st.focused = false; onWorld(); }

  // Drag (mouse and touch) scrolls the stage; a drag never counts as a click.
  stage.addEventListener("pointerdown", (e) => {
    if (st.mode !== "scroll" || e.button > 0) return;
    st.drag = { id: e.pointerId, x: e.clientX, y: e.clientY, sx: stage.scrollLeft, sy: stage.scrollTop, moved: false };
  });
  stage.addEventListener("pointermove", (e) => {
    const d = st.drag;
    if (!d || d.id !== e.pointerId) return;
    const dx = e.clientX - d.x, dy = e.clientY - d.y;
    if (!d.moved && Math.hypot(dx, dy) < 6) return;
    if (!d.moved) { d.moved = true; stage.setPointerCapture(e.pointerId); stage.classList.add("dragging"); }
    stage.scrollLeft = d.sx - dx; stage.scrollTop = d.sy - dy;
  });
  const endDrag = (e) => {
    const d = st.drag;
    if (!d || d.id !== e.pointerId) return;
    st.drag = null; stage.classList.remove("dragging");
    if (d.moved) { st.justDragged = true; setTimeout(() => { st.justDragged = false; }, 0); }
  };
  stage.addEventListener("pointerup", endDrag);
  stage.addEventListener("pointercancel", endDrag);
  stage.addEventListener("click", (e) => { if (st.justDragged) { e.stopPropagation(); e.preventDefault(); } }, true);

  // Minimap: the whole town scaled down, the current view framed; tap to go there.
  mini.addEventListener("click", (e) => {
    const r = mini.getBoundingClientRect();
    const t = V.minimapToTile((e.clientX - r.left) * mini.width / r.width, (e.clientY - r.top) * mini.height / r.height, st.mScale);
    centerOn(t.x, t.y);
  });
  function drawMinimap() {
    if (st.mode === "scroll" && room()) {
      mctx.imageSmoothingEnabled = true;
      mctx.drawImage(cv, 0, 0, mini.width, mini.height);
      const c = content();
      const f = V.minimapFrame(scrollPos(), st.tile, stage.clientWidth, stage.clientHeight, c.w, c.h, st.mScale);
      mctx.strokeStyle = "#ffd84a"; mctx.lineWidth = 2;
      mctx.strokeRect(f.x + 1, f.y + 1, Math.max(2, f.w - 2), Math.max(2, f.h - 2));
    }
    setTimeout(() => requestAnimationFrame(drawMinimap), 100);
  }

  // A phone keyboard opening/closing resizes the window height only; while the
  // talk input has focus that must not shrink the map (it would re-tile and jump).
  let lastW = document.documentElement.clientWidth;
  window.addEventListener("resize", () => {
    const w = document.documentElement.clientWidth, typing = document.activeElement && document.activeElement.id === "talkText";
    if (typing && w === lastW && st.mode === "scroll") return;
    lastW = w; layout();
  });
  const talkIn = document.getElementById("talkText");
  if (talkIn) talkIn.addEventListener("blur", () => setTimeout(layout, 200));
  layout();
  drawMinimap();
  return { onWorld, setSelf, centerOn, layout, state: st };
})();
