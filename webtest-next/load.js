// Load a web-next-src/*.ts module under node: esbuild bundles it to CommonJS
// (types stripped, nostr-tools resolved from node_modules) and it is required
// from memory. NEXT_SRC=<dir> loads another copy (e.g. the code before a fix).
const path = require("node:path");
const Module = require("node:module");
const { buildSync } = require("esbuild");

const SRC = path.resolve(process.env.NEXT_SRC || path.join(__dirname, "../web-next-src"));
module.exports = function load(name) {
  const file = path.join(SRC, name + ".ts");
  const r = buildSync({ entryPoints: [file], bundle: true, format: "cjs", platform: "node", write: false, nodePaths: [path.join(__dirname, "../node_modules")] });
  const m = new Module(file, module);
  m.paths = Module._nodeModulePaths(path.join(__dirname, ".."));
  m._compile(r.outputFiles[0].text, file);
  return m.exports;
};
