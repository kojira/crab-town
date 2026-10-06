// The parts of nostr-tools the Pages viewer uses, bundled (scripts/build-web.mjs)
// into web/nostr-tools.js as the global NostrTools: no CDN at run time.
export { verifyEvent, generateSecretKey, finalizeEvent } from "nostr-tools/pure";
export { decode } from "nostr-tools/nip19";
