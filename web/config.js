// generated from web-src/config.ts by scripts/build-web.mjs; edit the .ts file, not this one
window.CRAB_NOSTR = {
  town: "1eed3e5e0c4c408bc3f8d1844a6ed362d2d298d64f7ffde31415bbf840725f6a",
  relays: ["wss://r.kojira.io", "wss://n.kojira.io", "wss://x.kojira.io"],
  roomSpot: { x: 29, y: 15 },
  // nostarou's bedroom (by the bed); the owner walks there with their own avatar
  // actors whose knocks reach someone (the town's CRAB_EXTGATE_ACTOR). Knocks on
  // other houses are still recorded as town events, and the menu says so.
  listening: ["nostarou"],
  // nostarou is connected through extgate and moves by itself
  blossom: "https://blossom.primal.net"
  // where the owner uploads chat images (BUD-02, NIP-07 signed)
};
