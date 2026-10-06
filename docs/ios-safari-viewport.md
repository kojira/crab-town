# iOS Safari: zoom on focus, the soft keyboard and the viewports

Notes for building phone UIs (the next-generation town viewer first). Each
point says where it comes from; anything without a source is marked
**(推測 / inferred)** and has to be checked on a real iPhone before relying on it.

Sources read on 2026-10-06 (WebKit source at `main` = `b2b4227`).

## 1. Automatic zoom when a text field gets focus

- **Condition.** On focus, WebKit zooms so the field's text reaches 16px:
  `scale = clamp(16 / fontSize, minimumScale, maximumScale)`, where `fontSize`
  is the focused element's *used* font size
  (`renderer->style().fontDescription().usedSize()`). A field whose used font
  size is **>= 16px** gets scale <= 1 and is not zoomed in; anything under
  16px is. [S1][S2]
  - It is the used size, not the number in the stylesheet: measure it with
    `getComputedStyle(el).fontSize` on the element that actually takes focus
    (`<input>`, `<textarea>`, `contenteditable`), not on its wrapper.
  - `transform: scale()` does not change the used font size, so a scaled-down
    16px field passes this check but looks smaller **(推測: not traced in the source)**.
- **iPhone only.** The zoom is applied only when
  `allowsUserScalingIgnoringAlwaysScalable && currentUserInterfaceIdiomIsSmallScreen()` [S3].
- **`maximum-scale` / `user-scalable`.** Since iOS 10, Safari ignores
  `user-scalable`, `minimum-scale` and `maximum-scale` for the user's own pinch
  zoom [S4]. The focus zoom, however, is clamped by
  `maximumScaleFactorIgnoringAlwaysScalable` and gated by
  `allowsUserScalingIgnoringAlwaysScalable` [S2][S3], i.e. by the page's meta
  values — which is why `maximum-scale=1` stops the focus zoom while pinch still
  works. It is a fallback, not a fix: it hides a too-small field instead of
  making it readable, and MDN / WCAG 1.4.4 (200 % text resize) warn against
  limiting zoom [S5][S6]. **Use >= 16px fields; do not ship `maximum-scale=1`
  or `user-scalable=no`.**
- **`-webkit-text-size-adjust`.** Controls text inflation (e.g. after rotating
  to landscape). Safari iOS supports it only with the `-webkit-` prefix
  [S7][S8]. Use `100%` (not `none`, which also blocks the user's text-size
  preference) so sizes stay what the CSS says.

## 2. The soft keyboard: visual viewport vs layout viewport

- **Safari on iOS / iPadOS resizes only the visual viewport** when the
  on-screen keyboard opens; the layout viewport (what `position:fixed`, `vh`
  and the initial containing block use) keeps its size [S9]. Chrome on Android
  moved to the same behaviour in Chrome 108 [S9].
- Consequences of this behaviour, as listed by the Chrome team [S9]:
  - viewport-relative units **do not change** while the keyboard is up;
  - elements sized to the full screen keep their size;
  - `position:fixed` elements stay where they are **and can be covered by the keyboard**.
- To show the focused field, the visual viewport pans over the layout
  viewport: on mobile `visualViewport.offsetTop/offsetLeft` change rather than
  `window.scrollY` [S10]. iOS may also scroll the document itself when it is
  scrollable **(推測: no primary source found for the exact rule)**.
- **Pinch zoom also shrinks the visual viewport** (`visualViewport.scale > 1`,
  smaller `width/height`) without touching the layout viewport [S9][S10]. A
  handler cannot tell keyboard from zoom by height alone; it has to look at `scale`.

## 3. `vh` / `svh` / `lvh` / `dvh`, and `interactive-widget`

- `svh` = smallest possible viewport (browser UI expanded), `lvh` = largest
  (UI retracted), `dvh` = the current one, changing as the toolbar
  shows/hides [S11][S12]. `vh` is pinned to `lvh` [S12].
- Safari supports `svh/lvh/dvh` (plus `*vw`, `*vmin/vmax`, logical forms)
  since **Safari / iOS 15.4** [S11][S8].
- **None of them shrink for the keyboard on iOS.** The spec allows UAs to treat
  on-screen keyboards as overlays that have "no effect on any of the
  viewport-percentage lengths" [S12], and Safari keeps viewport units unchanged
  with the keyboard up [S9]. `100dvh` solves the toolbar problem, **not** the
  keyboard problem.
- **`interactive-widget`** (`resizes-visual` / `resizes-content` /
  `overlays-content`) in the viewport meta is supported by Chrome Android 108+
  and Firefox Android 133+; **not by Safari (macOS or iOS)** according to MDN
  browser-compat-data [S13][S5]. `interactive-widget=resizes-content` does
  nothing on iPhone.
- So on iPhone the only signal for "space left above the keyboard" is
  `window.visualViewport` (`height`, `offsetTop`, `scale`).

## 4. When `visualViewport` `resize` / `scroll` fire

- Safari iOS supports `VisualViewport` and its `resize` / `scroll` events since 13 [S8].
- In "update the rendering" (once per frame, before animation frame
  callbacks) the UA runs the *resize steps* — fire `resize` at the
  `VisualViewport` if its `scale`, `width` or `height` changed since the last
  run — and then the *scroll steps*, which fire each pending `scroll` once (a
  target already pending is not queued twice) [S14][S15]. Each event therefore
  fires **at most once per rendering update**, after the change.
- `resize` fires for keyboard show/hide, pinch zoom (scale), the focus zoom of
  §1 and rotation; `scroll` fires when the visual viewport pans (Safari
  revealing the focused field, pinch-pan) [S10][S14].
- A handler whose writes make Safari pan or rescale again (e.g. sizing the
  whole page column to `visualViewport.height`, which moves the focused field,
  which Safari pans to reveal, which fires `scroll`, ...) can run every frame —
  visible as flicker **(推測: follows from S9/S10/S14; the loop itself has to be
  shown by measurement)**.

## Rules for the next viewer that follow from this

1. Every focusable text field has a used font size >= 16px, asserted with
   `getComputedStyle` in a test. No `maximum-scale` / `user-scalable=no`.
2. Size the page with CSS (`100dvh`, toolbar-safe). No CSS unit or meta reacts
   to the iOS keyboard; do not design as if one did.
3. If the input bar must sit above the keyboard: read `visualViewport` from its
   events (already once per frame), **ignore it while `visualViewport.scale !== 1`**
   (pinch / focus zoom), write only when the value changed by more than 1px, and
   write only something that does not move the focused field again (an inset /
   translate of the input bar) — never the size of the map or the whole page.
4. Do not infer "keyboard up" from focus alone (hardware keyboards, iPad);
   use `innerHeight - (vv.height + vv.offsetTop)`.

## Sources

- [S1] WebKit `Source/WebKit/UIProcess/API/ios/WKWebViewIOS.mm`, `_zoomToFocusRect:` (`webViewStandardFontSize = 16`) — https://github.com/WebKit/WebKit/blob/main/Source/WebKit/UIProcess/API/ios/WKWebViewIOS.mm
- [S2] WebKit `Source/WebKit/WebProcess/WebPage/ios/WebPageIOS.mm` (`nodeFontSize = ... usedSize()`, `maximumScaleFactorIgnoringAlwaysScalable`) — https://github.com/WebKit/WebKit/blob/main/Source/WebKit/WebProcess/WebPage/ios/WebPageIOS.mm
- [S3] WebKit `Source/WebKit/UIProcess/ios/WKContentViewInteraction.mm` (the `_zoomToFocusRect:` call and its `allowScaling`) — https://github.com/WebKit/WebKit/blob/main/Source/WebKit/UIProcess/ios/WKContentViewInteraction.mm
- [S4] WebKit blog, "New Interaction Behaviors in iOS 10" — https://webkit.org/blog/7367/new-interaction-behaviors-in-ios-10/
- [S5] MDN, `<meta name="viewport">` — https://developer.mozilla.org/en-US/docs/Web/HTML/Reference/Elements/meta/name/viewport
- [S6] W3C, Understanding WCAG 2.2 SC 1.4.4 Resize Text — https://www.w3.org/WAI/WCAG22/Understanding/resize-text.html
- [S7] MDN, `text-size-adjust` — https://developer.mozilla.org/en-US/docs/Web/CSS/text-size-adjust
- [S8] MDN browser-compat-data (`css/properties/text-size-adjust`, `css/types/length` viewport units, `api/VisualViewport`) — https://github.com/mdn/browser-compat-data
- [S9] Chrome for Developers, "Prepare for viewport resize behavior changes coming to Chrome on Android" (Interop 2022 viewport investigation) — https://developer.chrome.com/blog/viewport-resize-behavior
- [S10] MDN, `VisualViewport` — https://developer.mozilla.org/en-US/docs/Web/API/VisualViewport
- [S11] WebKit blog, "New WebKit Features in Safari 15.4" — https://webkit.org/blog/12445/new-webkit-features-in-safari-15-4/
- [S12] CSS Values and Units Level 4, viewport-percentage lengths — https://drafts.csswg.org/css-values-4/#viewport-relative-lengths
- [S13] MDN browser-compat-data, `html/elements/meta/name/viewport/interactive-widget.json` — https://github.com/mdn/browser-compat-data/blob/main/html/elements/meta/name/viewport/interactive-widget.json
- [S14] CSSOM View, resize / scroll steps — https://drafts.csswg.org/cssom-view/#resizing-viewports
- [S15] HTML, event loop "update the rendering" — https://html.spec.whatwg.org/multipage/webappapis.html#update-the-rendering
