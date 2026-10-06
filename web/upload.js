// generated from web-src/upload.ts by scripts/build-web.mjs; edit the .ts file, not this one
const CrabUpload = (() => {
  const MAX_BYTES = 5 * 1024 * 1024;
  const MAX_URL = 1024;
  const TYPES = ["image/png", "image/jpeg", "image/gif", "image/webp"];
  const KIND_BLOSSOM_AUTH = 24242;
  function safeImageUrl(s) {
    if (typeof s !== "string" || !s || s.length > MAX_URL || /[\s"'<>\\\u0000-\u001f\u007f]/.test(s)) return null;
    let u;
    try {
      u = new URL(s);
    } catch {
      return null;
    }
    if (u.protocol !== "https:" || !u.hostname || u.username || u.password) return null;
    return s;
  }
  function checkFile(f) {
    if (!f) return "\u30D5\u30A1\u30A4\u30EB\u304C\u9078\u3070\u308C\u3066\u3044\u306A\u3044";
    if (!TYPES.includes(f.type)) return "\u753B\u50CF\uFF08PNG / JPEG / GIF / WebP\uFF09\u3060\u3051";
    if (f.size > MAX_BYTES) return "5MB \u307E\u3067";
    if (f.size === 0) return "\u7A7A\u306E\u30D5\u30A1\u30A4\u30EB";
    return "";
  }
  function authTemplate(sha256hex, now, ttl = 300) {
    return {
      kind: KIND_BLOSSOM_AUTH,
      created_at: now,
      content: "crab-town: upload an image to the chat",
      tags: [["t", "upload"], ["x", sha256hex], ["expiration", String(now + ttl)]]
    };
  }
  function authHeader(ev) {
    const bytes = new TextEncoder().encode(JSON.stringify(ev));
    let bin = "";
    for (const b of bytes) bin += String.fromCharCode(b);
    return "Nostr " + btoa(bin);
  }
  function pickBlobUrl(body, sha256hex) {
    if (!body || typeof body !== "object") return null;
    const d = body;
    if (typeof d.sha256 === "string" && d.sha256 !== sha256hex) return null;
    return safeImageUrl(d.url);
  }
  const hex = (buf) => [...new Uint8Array(buf)].map((b) => b.toString(16).padStart(2, "0")).join("");
  async function upload(server, file, deps) {
    const bad = checkFile(file);
    if (bad) throw new Error(bad);
    const base = safeImageUrl(server.replace(/\/+$/, ""));
    if (!base) throw new Error("\u30A2\u30C3\u30D7\u30ED\u30FC\u30C9\u5148 (blossom) \u304C https \u3067\u306A\u3044");
    const data = await file.arrayBuffer();
    const sha = hex(await deps.digest(data));
    const ev = await deps.sign(authTemplate(sha, Math.floor((deps.now ? deps.now() : Date.now()) / 1e3)));
    let res;
    try {
      res = await deps.fetch(base + "/upload", {
        method: "PUT",
        body: data,
        headers: { Authorization: authHeader(ev), "Content-Type": file.type, "X-SHA-256": sha }
      });
    } catch (e) {
      throw new Error("\u30A2\u30C3\u30D7\u30ED\u30FC\u30C9\u5148\u306B\u3064\u306A\u304C\u3089\u306A\u3044: " + (e instanceof Error && e.message || e));
    }
    if (!res.ok) throw new Error(`\u30A2\u30C3\u30D7\u30ED\u30FC\u30C9\u3092\u65AD\u3089\u308C\u305F (${res.status}${res.headers.get("X-Reason") ? " " + res.headers.get("X-Reason") : ""})`);
    let body;
    try {
      body = await res.json();
    } catch {
      body = null;
    }
    const url = pickBlobUrl(body, sha);
    if (!url) throw new Error("\u30A2\u30C3\u30D7\u30ED\u30FC\u30C9\u5148\u306E\u8FD4\u4E8B\u306B\u4F7F\u3048\u308B https \u306E URL \u304C\u7121\u3044");
    return url;
  }
  function imageNode(doc, url) {
    const safe = safeImageUrl(url);
    if (!safe) return null;
    const a = doc.createElement("a");
    a.className = "img";
    a.href = safe;
    a.target = "_blank";
    a.rel = "noopener noreferrer";
    const img = doc.createElement("img");
    img.alt = "\u753B\u50CF";
    img.loading = "lazy";
    img.decoding = "async";
    img.referrerPolicy = "no-referrer";
    img.src = safe;
    img.onerror = () => {
      a.replaceChildren("\uFF08\u753B\u50CF\u3092\u8AAD\u307F\u8FBC\u3081\u306A\u304B\u3063\u305F\uFF09");
    };
    a.append(img);
    return a;
  }
  return { MAX_BYTES, TYPES, KIND_BLOSSOM_AUTH, safeImageUrl, checkFile, authTemplate, authHeader, pickBlobUrl, upload, imageNode };
})();
if (typeof module !== "undefined") module.exports = CrabUpload;
