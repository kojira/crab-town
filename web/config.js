// Where the Pages viewer (nostr.html) finds the town. Both can be overridden
// on the URL: nostr.html?town=<hex|npub>&relays=wss://a,wss://b
// town = crab-town's own pubkey (print it with `crab-town nostr-pubkey`).
window.CRAB_NOSTR = {
  town: "1eed3e5e0c4c408bc3f8d1844a6ed362d2d298d64f7ffde31415bbf840725f6a",
  relays: ["wss://r.kojira.io", "wss://n.kojira.io", "wss://x.kojira.io"],
  roomSpot: { x: 29, y: 15 }, // nostarou's bedroom (by the bed)
  ownerActor: "nostarou",     // the actor the town owner drives (CRAB_NOSTR_OWNER_ACTOR on the town side)
};
