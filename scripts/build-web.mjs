// Build the viewer: web-src/*.ts -> web/*.js (committed, so `go build` /
// `go test` need no node). The viewer stays a set of classic scripts sharing
// one global scope, so each file is only transformed (types stripped), not
// bundled. nostr-tools is the one bundle: web-src/vendor/nostr-tools.ts ->
// web/nostrtools.js (global NostrTools), so no script comes from a CDN.
//   node scripts/build-web.mjs
import { build, transform } from "esbuild";
import { readFile, readdir, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const src = path.join(root, "web-src"), out = path.join(root, "web");
const TARGET = "es2022";
const header = (f) => `// generated from web-src/${f} by scripts/build-web.mjs; edit the .ts file, not this one\n`;

const files = (await readdir(src)).filter((f) => f.endsWith(".ts") && !f.endsWith(".d.ts")).sort();
for (const f of files) {
  const code = await readFile(path.join(src, f), "utf8");
  // nostr.ts is loaded as type="module" (its names stay out of the global scope)
  const r = await transform(code, { loader: "ts", target: TARGET, format: f === "nostr.ts" ? "esm" : undefined, sourcefile: f, legalComments: "inline" });
  await writeFile(path.join(out, f.replace(/\.ts$/, ".js")), header(f) + r.code);
}

await build({
  entryPoints: [path.join(src, "vendor", "nostr-tools.ts")],
  outfile: path.join(out, "nostrtools.js"),
  bundle: true, format: "iife", globalName: "NostrTools", target: TARGET, minify: true,
  legalComments: "eof", logLevel: "warning",
  banner: { js: "// generated from web-src/vendor/nostr-tools.ts (nostr-tools, bundled) by scripts/build-web.mjs" },
});
console.log(`built ${files.length} scripts + nostrtools.js`);
