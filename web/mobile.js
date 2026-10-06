// generated from web-src/mobile.ts by scripts/build-web.mjs; edit the .ts file, not this one
const CrabView = (() => {
  const V = CrabViewport;
  const stage = document.getElementById("stage");
  const mini = document.getElementById("minimap"), mctx = mini.getContext("2d");
  const jumps = document.getElementById("jumps");
  const st = { mode: "fit", tile: T, focused: false, self: null, jumpKey: "", drag: null, justDragged: false, mScale: 2 };
  const room = () => Object.values(rooms)[0];
  const townW = () => room()?.width ?? cv.width / T;
  const townH = () => room()?.height ?? cv.height / T;
  const content = () => ({ w: townW() * st.tile, h: townH() * st.tile });
  const scrollPos = () => ({ x: stage.scrollLeft, y: stage.scrollTop });
  function scrollTo(o) {
    const c = content(), v = V.clampView(o.x, o.y, stage.clientWidth, stage.clientHeight, c.w, c.h);
    stage.scrollLeft = Math.max(0, v.x);
    stage.scrollTop = Math.max(0, v.y);
  }
  function centerOn(tx, ty) {
    const c = content();
    scrollTo(V.centerOn(tx, ty, st.tile, stage.clientWidth, stage.clientHeight, c.w, c.h));
  }
  let laidKey = "";
  function layout() {
    const cw = document.documentElement.clientWidth;
    const key = cw + "x" + townW() + "x" + townH();
    if (st.mode === "scroll" && key === laidKey) return;
    laidKey = key;
    const availW = cw - 16;
    const mode = V.chooseMode(availW, townW());
    if (mode !== st.mode) {
      st.mode = mode;
      document.body.classList.toggle("narrow", mode === "scroll");
    }
    if (mode === "fit") {
      cv.style.width = cv.style.height = stage.style.height = "";
      window.LABEL_SCALE = 1;
      return;
    }
    const appH = document.body.clientHeight || window.innerHeight;
    const top = stage.getBoundingClientRect().top - document.body.getBoundingClientRect().top;
    const stageH = V.phoneStageHeight(appH, top, townH() * V.SCROLL_MIN_TILE);
    const keep = scrollPos(), old = st.tile;
    st.tile = V.scrollTile(stageH, townH());
    window.LABEL_SCALE = V.labelScale(st.tile);
    cv.style.width = townW() * st.tile + "px";
    cv.style.height = townH() * st.tile + "px";
    stage.style.height = stageH + "px";
    const c = content();
    st.mScale = V.minimapScale(townW(), townH(), Math.min(140, stage.clientWidth * 0.4), 60);
    const mw = townW() * st.mScale, mh = townH() * st.mScale;
    if (mini.width !== mw) mini.width = mw;
    if (mini.height !== mh) mini.height = mh;
    if (old !== st.tile) {
      const k = st.tile / old, hw = stage.clientWidth / 2, hh = stage.clientHeight / 2;
      scrollTo({ x: (keep.x + hw) * k - hw, y: (keep.y + hh) * k - hh });
    }
    if (c.w <= stage.clientWidth) scrollTo({ x: 0, y: 0 });
  }
  function onWorld() {
    const r = room();
    if (!r) return;
    layout();
    if (!st.focused && st.mode === "scroll") {
      const f = V.initialFocus(r, actors, st.self);
      if (f) {
        centerOn(f.x, f.y);
        st.focused = true;
      }
    }
    const ts = V.jumpTargets(r, actors, st.self);
    const key = ts.map((t) => t.id + t.label + t.x + "," + t.y).join("|");
    if (key === st.jumpKey) return;
    st.jumpKey = key;
    jumps.replaceChildren(...ts.map((t) => {
      const b = document.createElement("button");
      b.type = "button";
      b.textContent = t.label;
      b.dataset.target = t.id;
      b.onclick = () => centerOn(t.x, t.y);
      return b;
    }));
  }
  function setSelf(id) {
    st.self = id || null;
    st.focused = false;
    onWorld();
  }
  stage.addEventListener("pointerdown", (e) => {
    if (st.mode !== "scroll" || e.button > 0) return;
    st.drag = { id: e.pointerId, x: e.clientX, y: e.clientY, sx: stage.scrollLeft, sy: stage.scrollTop, moved: false };
  });
  stage.addEventListener("pointermove", (e) => {
    const d = st.drag;
    if (!d || d.id !== e.pointerId) return;
    const dx = e.clientX - d.x, dy = e.clientY - d.y;
    if (!d.moved && Math.hypot(dx, dy) < 6) return;
    if (!d.moved) {
      d.moved = true;
      stage.setPointerCapture(e.pointerId);
      stage.classList.add("dragging");
    }
    stage.scrollLeft = d.sx - dx;
    stage.scrollTop = d.sy - dy;
  });
  const endDrag = (e) => {
    const d = st.drag;
    if (!d || d.id !== e.pointerId) return;
    st.drag = null;
    stage.classList.remove("dragging");
    if (d.moved) {
      st.justDragged = true;
      setTimeout(() => {
        st.justDragged = false;
      }, 0);
    }
  };
  stage.addEventListener("pointerup", endDrag);
  stage.addEventListener("pointercancel", endDrag);
  stage.addEventListener("click", (e) => {
    if (st.justDragged) {
      e.stopPropagation();
      e.preventDefault();
    }
  }, true);
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
      mctx.strokeStyle = "#ffd84a";
      mctx.lineWidth = 2;
      mctx.strokeRect(f.x + 1, f.y + 1, Math.max(2, f.w - 2), Math.max(2, f.h - 2));
    }
    setTimeout(() => requestAnimationFrame(drawMinimap), 100);
  }
  window.addEventListener("resize", () => {
    layout();
  });
  const talkIn = document.getElementById("talkText");
  if (talkIn) talkIn.addEventListener("blur", () => setTimeout(layout, 200));
  layout();
  drawMinimap();
  return { onWorld, setSelf, centerOn, layout, state: st };
})();
