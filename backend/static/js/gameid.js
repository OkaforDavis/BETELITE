// "Add your game ID" sheet, used wherever a game name is required first.
import { put } from './api.js';
import { store } from './store.js';
import { html, ic, sheet, sheetHead, busy, toast } from './ui.js';

export function gameIdSheet(gameId, onSaved) {
  const game = store.get().games.find((g) => g.id === gameId);
  const existing = store.get().profile.gameProfiles.find((g) => g.game === gameId);
  sheet(String(html`${sheetHead(game.name + ' ID', 'Used to verify your match screenshots')}
    <div class="notice brand" style="margin-bottom:16px">${ic('info')}<div>${game.nameHint}</div></div>
    <form id="gid">
      <label class="field"><span class="label">In-game name</span>
        <input class="input" name="gamertag" maxlength="32" required value="${existing?.gamertag || ''}" autocomplete="off" autocapitalize="off" spellcheck="false"></label>
      <label class="field"><span class="label">In-game ID number <span class="faint">(optional)</span></span>
        <input class="input" name="inGameId" maxlength="40" value="${existing?.inGameId || ''}" autocomplete="off" inputmode="text"></label>
      <div class="notice warn" style="margin-top:16px">${ic('lock')}<div class="small">Your name is <b>locked after your first match</b> to prevent cheating. Double-check the spelling.</div></div>
      <div class="sheet-actions"><button class="btn primary block" type="submit">Save</button></div>
    </form>`), {
    label: 'Game ID',
    onMount(root, close) {
      root.querySelector('#gid').onsubmit = (e) => {
        e.preventDefault();
        const f = new FormData(e.target);
        busy(e.target.querySelector('[type=submit]'), async () => {
          const { gameProfiles } = await put('/me/games/' + gameId, { gamertag: f.get('gamertag'), inGameId: f.get('inGameId') });
          store.set({ profile: { ...store.get().profile, gameProfiles } });
          toast(game.short + ' ID saved');
          close();
          onSaved?.();
        });
      };
    },
  });
}
