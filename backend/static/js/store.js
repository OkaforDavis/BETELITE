// Tiny shared state with change subscriptions.
const state = {
  user: null,       // Firebase user
  profile: null,    // /api/me profile (balance, game IDs, consent, ...)
  games: [],        // supported games
  unread: 0,        // unread notifications
  current: null,    // the match I should act on now
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
