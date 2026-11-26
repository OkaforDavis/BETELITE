// Tiny shared state with change subscriptions.
const state = {
  user: null,       // Firebase user
  profile: null,    // /api/me profile (balance, game IDs, consent, ...)
  games: [],        // supported games
  unread: 0,        // unread notifications
  current: null,    // the match I should act on now
  stakingEnabled: false, // real-money stakes (off until licensed)
  bountyEnabled: false,  // bounty 1v1s: only the host puts up a prize
  hostCupsEnabled: false, // player-hosted cups
  hostFeePercent: 20,
  location: null,   // where the player is now ({country, currency})
  homeSwitch: null, // whether they can move their wallet to that country
  payCurrencies: [], // currencies payable right now (e.g. NGN, GHS)
  online: navigator.onLine,
};
const subs = new Set();

export const store = {
  get: () => state,
  set(patch) {
    Object.assign(state, patch);
    subs.forEach((fn) => fn(state));
  },
  subscribe(fn) {
    subs.add(fn);
    return () => subs.delete(fn);
  },
};

export const gameName = (id) => state.games.find((g) => g.id === id)?.short || id;
// Admin powers of the signed-in player: cups, disputes, withdrawals, support,
// users, money; "admins" = the owner.
export const can = (perm) => !!state.profile?.adminPerms?.includes(perm);
export const gameInfo = (id) => state.games.find((g) => g.id === id) || {};
// What to screenshot for a game, e.g. "full-time result screen".
export const resultScreen = (id) => gameInfo(id).resultScreen || 'final result screen';
