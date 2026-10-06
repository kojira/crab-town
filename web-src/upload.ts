// crab-town viewer: images in the chat (issue #20, owner only).
// The owner picks a file, it goes to a Blossom server (BUD-02 PUT /upload,
// authorized by a kind 24242 event the owner signs with NIP-07), and the URL
// the server returns travels in a talk ({"type":"talk","image":url}). The town
// refuses an image from anyone but the owner; the viewer only hides the button.
// Pure parts (safeImageUrl, checkFile, authTemplate, authHeader, pickBlobUrl)
// are tested by node --test; upload() takes its fetch / signer / digest as arguments.
const CrabUpload = (() => {
  const MAX_BYTES = 5 * 1024 * 1024; // refuse bigger files before uploading
  const MAX_URL = 1024; // the town refuses longer (world.MaxImageURL)
  const TYPES = ["image/png", "image/jpeg", "image/gif", "image/webp"];
  const KIND_BLOSSOM_AUTH = 24242;

  // The only image URLs drawn: absolute https, a host, no credentials, no
  // whitespace / quotes / angle brackets / backslash. Anything else -> null.
  function safeImageUrl(s: unknown): string | null {
    if (typeof s !== "string" || !s || s.length > MAX_URL || /[\s"'<>\\\u0000-\u001f\u007f]/.test(s)) return null;
    let u: URL;
    try { u = new URL(s); } catch { return null; }
    if (u.protocol !== "https:" || !u.hostname || u.username || u.password) return null;
    return s;
  }

  // Whether a picked file may be uploaded; the reason in words when not.
  function checkFile(f: { size: number; type: string } | null | undefined): string {
    if (!f) return "ファイルが選ばれていない";
    if (!TYPES.includes(f.type)) return "画像（PNG / JPEG / GIF / WebP）だけ";
    if (f.size > MAX_BYTES) return "5MB まで";
    if (f.size === 0) return "空のファイル";
    return "";
  }

  // BUD-02 upload authorization: kind 24242, t=upload, x=sha256 of the body,
  // expiration a few minutes ahead. Signed by the owner (NIP-07).
  function authTemplate(sha256hex: string, now: number, ttl = 300) {
    return { kind: KIND_BLOSSOM_AUTH, created_at: now, content: "crab-town: upload an image to the chat",
      tags: [["t", "upload"], ["x", sha256hex], ["expiration", String(now + ttl)]] };
  }
  type AuthTemplate = ReturnType<typeof authTemplate>;
  // "Nostr <base64 of the signed event JSON>" (UTF-8 safe)
  function authHeader(ev: object): string {
    const bytes = new TextEncoder().encode(JSON.stringify(ev));
    let bin = "";
    for (const b of bytes) bin += String.fromCharCode(b);
    return "Nostr " + btoa(bin);
  }
  // The blob descriptor the server answers with: its url, if it is one we draw.
  function pickBlobUrl(body: unknown, sha256hex: string): string | null {
    if (!body || typeof body !== "object") return null;
    const d = body as { url?: unknown; sha256?: unknown };
    if (typeof d.sha256 === "string" && d.sha256 !== sha256hex) return null; // not what we sent
    return safeImageUrl(d.url);
  }
  const hex = (buf: ArrayBuffer) => [...new Uint8Array(buf)].map(b => b.toString(16).padStart(2, "0")).join("");

  // what upload() needs from the page (passed in, so node --test can fake them)
  interface Deps {
    fetch: (url: string, init: RequestInit) => Promise<Response>;
    sign: (t: AuthTemplate) => Promise<object>;
    digest: (data: ArrayBuffer) => Promise<ArrayBuffer>;
    now?: () => number;
  }
  // Upload one file to server (https origin). Resolves to the image URL or
  // throws an Error whose message says why, in words.
  async function upload(server: string, file: Blob, deps: Deps): Promise<string> {
    const bad = checkFile(file);
    if (bad) throw new Error(bad);
    const base = safeImageUrl(server.replace(/\/+$/, ""));
    if (!base) throw new Error("アップロード先 (blossom) が https でない");
    const data = await file.arrayBuffer();
    const sha = hex(await deps.digest(data));
    const ev = await deps.sign(authTemplate(sha, Math.floor((deps.now ? deps.now() : Date.now()) / 1000)));
    let res: Response;
    try {
      res = await deps.fetch(base + "/upload", { method: "PUT", body: data,
        headers: { Authorization: authHeader(ev), "Content-Type": file.type, "X-SHA-256": sha } });
    } catch (e) { throw new Error("アップロード先につながらない: " + ((e instanceof Error && e.message) || e)); }
    if (!res.ok) throw new Error(`アップロードを断られた (${res.status}${res.headers.get("X-Reason") ? " " + res.headers.get("X-Reason") : ""})`);
    let body: unknown;
    try { body = await res.json(); } catch { body = null; }
    const url = pickBlobUrl(body, sha);
    if (!url) throw new Error("アップロード先の返事に使える https の URL が無い");
    return url;
  }

  // The image in a chat line: a link to the full image wrapping a size-capped
  // <img>. Built with DOM properties only (no innerHTML); an unsafe URL gives null.
  function imageNode(doc: Document, url: string | undefined): HTMLAnchorElement | null {
    const safe = safeImageUrl(url);
    if (!safe) return null;
    const a = doc.createElement("a");
    a.className = "img"; a.href = safe; a.target = "_blank"; a.rel = "noopener noreferrer";
    const img = doc.createElement("img");
    img.alt = "画像"; img.loading = "lazy"; img.decoding = "async"; img.referrerPolicy = "no-referrer";
    img.src = safe;
    img.onerror = () => { a.replaceChildren("（画像を読み込めなかった）"); };
    a.append(img);
    return a;
  }

  return { MAX_BYTES, TYPES, KIND_BLOSSOM_AUTH, safeImageUrl, checkFile, authTemplate, authHeader, pickBlobUrl, upload, imageNode };
})();
if (typeof module !== "undefined") module.exports = CrabUpload;
