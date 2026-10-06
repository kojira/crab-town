// town-next viewer: the HTTP API, signed with NIP-98 (kind 27235) through a
// NIP-07 signer (window.nostr). An agent signs the same events with its own
// key; the viewer is only a thin layer over these calls (docs/town-next-ui.md 0.2).
import { getToken } from "nostr-tools/nip98";

export interface Signer {
  getPublicKey(): Promise<string>;
  signEvent(t: { kind: number; created_at: number; tags: string[][]; content: string }): Promise<any>;
}
export interface ApiResult { ok: boolean; status: number; error?: string; you?: string; rights?: any }

// The operations of the existing town-next API this viewer uses.
export type Op =
  | { path: "/join" }
  | { path: "/move"; body: { x: number; y: number } }
  | { path: "/interact"; body: { furniture: string } }
  | { path: "/say"; body: { text: string } }
  | { path: "/plots/apply"; body: { house: string } };

// authHeader: "Nostr <base64 event>" for method + base + path. With a body the
// payload tag is sha256(JSON.stringify(body)) -- exactly the bytes sent.
export function authHeader(signer: Signer, base: string, method: string, path: string, body?: object): Promise<string> {
  const url = base.replace(/\/+$/, "") + path;
  return getToken(url, method, (e) => signer.signEvent(e), true, body);
}

export async function call(signer: Signer, base: string, op: Op, fetchFn: typeof fetch = fetch): Promise<ApiResult> {
  const body = "body" in op ? op.body : undefined;
  const auth = await authHeader(signer, base, "POST", op.path, body);
  const res = await fetchFn(base.replace(/\/+$/, "") + op.path, {
    method: "POST",
    headers: body ? { Authorization: auth, "Content-Type": "application/json" } : { Authorization: auth },
    body: body ? JSON.stringify(body) : undefined,
  });
  let j: any = {};
  try { j = await res.json(); } catch { /* not JSON */ }
  return { ok: res.ok && j.ok !== false, status: res.status, error: j.error, you: j.you, rights: j.rights };
}

// worldURL: the /world WebSocket; with a signer, ?auth= carries a GET /world
// event (a browser WebSocket cannot send headers).
export async function worldURL(base: string, signer?: Signer): Promise<string> {
  const ws = base.replace(/\/+$/, "").replace(/^http/, "ws") + "/world";
  if (!signer) return ws;
  return ws + "?auth=" + encodeURIComponent(await authHeader(signer, base, "GET", "/world"));
}
