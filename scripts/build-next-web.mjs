// Build the town-next viewer: web-next-src/main.ts (+ core.ts, api.ts and the
// nostr-tools parts it imports) -> internal/nextweb/static/app.js as one
// bundle, plus index.html and app.css copied as they are. The output is
// committed, so `go build` / `go test` need no node (like web/). The current
// town's viewer (web-src -> web/) is built by build-web.mjs and is untouched.
//   node scripts/build-next-web.mjs
import { build } from "esbuild";
import { copyFile, mkdir } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const src = path.join(root, "web-next-src"), out = path.join(root, "internal", "nextweb", "static");
await mkdir(out, { recursive: true });
await build({
  entryPoints: [path.join(src, "main.ts")],
  outfile: path.join(out, "app.js"),
  bundle: true, format: "iife", target: "es2022", minify: true,
  legalComments: "eof", logLevel: "warning",
  banner: { js: "// generated from web-next-src/ by scripts/build-next-web.mjs; edit the .ts files, not this one" },
});
for (const f of ["index.html", "app.css"]) await copyFile(path.join(src, f), path.join(out, f));
console.log("built internal/nextweb/static (app.js, index.html, app.css)");
