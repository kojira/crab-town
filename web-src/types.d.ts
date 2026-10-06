// Shared types of the viewer. The web-src/*.ts files (except nostr.ts) are
// classic scripts: they share one global scope in the page, exactly like the
// web/*.js files they compile to, so these declarations are global too.

// ---- world shapes (JSON of internal/world: Room, Zone, House, Furniture, Actor) ----
interface Pos { x: number; y: number }
interface Size { w: number; h: number }
interface Rect { x: number; y: number; w: number; h: number }
interface Zone { id: string; name: string; floor: string; visibility: string; rect: Rect; house?: string; private?: boolean }
interface House { id: string; owner: string; invited?: string[] | null; rect: Rect }
interface Furniture {
  id: string; kind: string; label: string; function: string;
  pos: Pos; size?: Size; access?: Pos; seat?: Pos; state?: string; walkable?: boolean;
}
interface Room {
  id: string; width: number; height: number; visibility?: string;
  houses?: House[] | null; zones?: Zone[] | null; walls?: Rect[] | null; doors?: Pos[] | null;
  furniture: Furniture[];
  hidden_zones?: string[]; in_use?: string[];
}
interface Actor {
  id: string; name: string; room: string; pos: Pos; state: string;
  using?: string; target?: Pos; hidden?: boolean; pubkey?: string; role?: string;
}

// ---- world messages (/world WebSocket, or kind 23411 content on Nostr) ----
// at / created_at: the Nostr event time; replay: a line restored from localStorage.
interface MsgMeta { at?: number; created_at?: number; replay?: boolean }
interface SnapshotMsg extends MsgMeta { type: "snapshot"; viewer?: string; rooms: Room[]; actors: Actor[] }
interface ActorMsg extends MsgMeta { type: "actor"; actor?: Actor }
interface InteractMsg extends MsgMeta { type: "interact"; actor: Actor; furniture: Furniture }
interface OccupancyMsg extends MsgMeta { type: "occupancy"; room: string; in_use?: string[]; hidden_zones?: string[] }
interface SayMsg extends MsgMeta { type: "say"; actor?: Actor; message?: string; reply_to?: string }
interface TalkMsg extends MsgMeta { type: "talk"; by?: string; role?: string; to?: string; message?: string }
interface KnockMsg extends MsgMeta { type: "knock"; by?: string; room?: string; house?: string; message?: string }
interface ResultMsg extends MsgMeta { type: "result"; cmd: string; ok: boolean; error?: string; role?: string; p?: string; e?: string }
type WorldMsg = SnapshotMsg | ActorMsg | InteractMsg | OccupancyMsg | SayMsg | TalkMsg | KnockMsg | ResultMsg;

// ---- drawing ----
// d(x, y, w, h, color): a rectangle in sprite dots (1 tile = 16x16 dots)
type DotPen = (x: number, y: number, w: number, h: number, c: string) => void;
type FurniturePainter = (d: DotPen, W: number, H: number) => void;
type FloorPainter = (d: DotPen, x: number, y: number, z: Zone) => void;
interface Sprite { body: string[]; legs: string[][]; pal: Record<string, string> }
// per-actor render state: walking interpolation and the walk frame
interface ActorView { from: Pos; to: Pos; start: number; flip: boolean; frame: number }

// ---- page globals ----
interface CrabNostrConfig {
  town?: string; relays?: string[]; roomSpot?: Pos; listening?: string[]; talkTo?: string;
}
// NIP-07 signer (browser extension)
interface Nip07 {
  getPublicKey(): Promise<string>;
  signEvent(t: { kind: number; created_at: number; tags: string[][]; content: string }): Promise<{
    id: string; pubkey: string; sig: string; kind: number; created_at: number; tags: string[][]; content: string;
  }>;
}
interface Window {
  CRAB_NOSTR?: CrabNostrConfig;
  LABEL_SCALE?: number; // set by mobile.js: canvas labels are enlarged when tiles are shown small
  nostr?: Nip07;
}
// CommonJS export for `node --test` (the pure parts of chatui / chatlog / viewport / avatar)
declare var module: { exports: unknown } | undefined;

// ---- Nostr ----
interface NostrEvent { id: string; pubkey: string; sig: string; kind: number; created_at: number; tags: string[][]; content: string }
type VerifyFn = (ev: NostrEvent) => boolean;
