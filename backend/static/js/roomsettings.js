// Room settings for games that have them (Call of Duty: Mobile): mode, map,
// score and time limit, loadout rules.
//  - settingsBox(): one-line summary with a "See all settings" fold, for
//    cups, challenges and matches. Keeps pages short.
//  - settingsPicker() + readSettings(): the form group used when posting a
//    challenge or creating a cup. Folded by default, showing the summary.
import { gameInfo } from './store.js';
import { html, ic } from './ui.js';

const specOf = (game) => gameInfo(game)?.settings || null;
export const hasSettings = (game) => !!specOf(game);

function modeOf(spec, id) { return spec?.modes.find((m) => m.id === id); }

export function summaryText(game, s) {
  const spec = specOf(game); const m = modeOf(spec, s?.mode);
  if (!m) return '';
  const parts = [m.name, `First to ${s.scoreLimit} ${m.scoreLabel}`];
  if (s.timeLimit) parts.push(`${s.timeLimit} min`);
  if (s.map && s.map !== 'Host picks') parts.push(s.map);
  return parts.join(' · ');
}

// Summary + "See all settings" (details). Shown to everyone.
export function settingsBox(game, s, { title = 'Match settings', open = false } = {}) {
  const spec = specOf(game); const m = modeOf(spec, s?.mode);
  if (!m) return '';
  const rules = (s.rules || []).map((id) => spec.rules.find((r) => r.id === id)?.label).filter(Boolean);
  return html`<div class="settings-box">
    <div class="row" style="gap:10px"><div class="icon-tile brand">${ic('target')}</div>
      <div class="grow" style="min-width:0"><div class="tiny faint">${title}</div><b class="small">${summaryText(game, s)}</b></div></div>
    <details ${open ? 'open' : ''}><summary class="small">${ic('chevronDown')} See all settings</summary>
      <div class="list" style="margin-top:10px">
        <div class="list-row"><div class="grow muted small">Mode</div><b class="small">${m.name}</b></div>
        <div class="list-row"><div class="grow muted small">To win</div><b class="small">First to ${s.scoreLimit} ${m.scoreLabel}</b></div>
        ${s.timeLimit ? html`<div class="list-row"><div class="grow muted small">Time limit</div><b class="small">${s.timeLimit} minutes (${s.timeLimit * 60} seconds)</b></div>` : ''}
        <div class="list-row"><div class="grow muted small">Map</div><b class="small">${s.map || 'Host picks'}</b></div>
        <div class="list-row" style="align-items:flex-start"><div class="grow muted small">Loadout</div>
          <div style="text-align:right">${rules.length ? rules.map((r) => html`<div class="small"><b>${r}</b></div>`) : html`<b class="small">Anything allowed</b>`}</div></div>
      </div>
      ${m.tip ? html`<p class="small muted" style="margin-top:10px">${m.tip}${s.timeLimit ? ' If time runs out first, the highest score wins.' : ''}</p>` : ''}
      <p class="tiny faint" style="margin-top:6px">The host sets these in the game's room settings before starting. Breaking the loadout rules can lose you the match.</p>
    </details></div>`;
}

// Form group for picking settings. size: '1v1' or 'ffa'.
export function settingsPicker(game, size, current) {
  const spec = specOf(game);
  if (!spec) return '';
  const modes = spec.modes.filter((m) => m.sizes.includes(size));
  const s = current?.mode && modes.some((m) => m.id === current.mode) ? current : defaults(modes[0]);
  return html`<details class="settings-pick" id="settings-pick" ${size === 'ffa' ? 'open' : ''}>
    <summary><div class="grow" style="min-width:0"><div class="label" style="margin:0">Match settings</div>
      <div class="small brand-text ellipsis" id="settings-sum">${summaryText(game, s)}</div></div><span class="small muted">Change ${ic('chevronDown')}</span></summary>
    <div class="settings-body">
      <div class="field" style="margin-top:4px"><span class="label">Mode</span>
        <div class="chips" style="flex-wrap:wrap">${modes.map((m) => html`<button type="button" class="chip ${m.id === s.mode ? 'active' : ''}" data-mode="${m.id}">${m.name}</button>`)}</div>
        <p class="hint" id="mode-tip">${modeOf(spec, s.mode)?.tip || ''}</p></div>
      <div class="grid-2">
        <label class="field"><span class="label" id="score-label">First to (${modeOf(spec, s.mode)?.scoreLabel})</span>
          <input class="input num" name="scoreLimit" type="number" min="1" max="${modeOf(spec, s.mode)?.maxScore}" value="${s.scoreLimit}"></label>
        <label class="field" style="margin-top:0" id="time-field" ${s.timeLimit ? '' : 'hidden'}><span class="label">Time limit (minutes)</span>
          <input class="input num" name="timeLimit" type="number" min="1" max="30" value="${s.timeLimit || 10}"></label></div>
      <label class="field"><span class="label">Map</span><select class="select" name="map">${spec.maps.map((mp) => html`<option ${mp === s.map ? 'selected' : ''}>${mp}</option>`)}</select></label>
      <div class="field"><span class="label">Loadout rules</span>
        <div class="rule-grid">${spec.rules.map((r) => html`<label class="check"><input type="checkbox" name="rule" value="${r.id}" ${(s.rules || []).includes(r.id) ? 'checked' : ''}><span>${r.label}</span></label>`)}</div></div>
    </div></details>`;
}

function defaults(m) { return { mode: m.id, scoreLimit: m.score, timeLimit: m.time, map: 'Host picks', rules: [] }; }

// Wires the picker inside root; returns a function that reads the settings.
export function bindSettingsPicker(root, game) {
  const box = root.querySelector('#settings-pick');
  if (!box) return () => null;
  const spec = specOf(game);
  let mode = box.querySelector('[data-mode].active')?.dataset.mode;
  const read = () => ({
    mode,
    scoreLimit: Number(box.querySelector('[name=scoreLimit]').value) || 0,
    timeLimit: box.querySelector('#time-field').hidden ? 0 : Number(box.querySelector('[name=timeLimit]').value) || 0,
    map: box.querySelector('[name=map]').value,
    rules: [...box.querySelectorAll('[name=rule]:checked')].map((x) => x.value),
  });
  const refresh = () => { box.querySelector('#settings-sum').textContent = summaryText(game, read()); };
  box.querySelectorAll('[data-mode]').forEach((b) => b.addEventListener('click', () => {
    mode = b.dataset.mode;
    const m = modeOf(spec, mode);
    box.querySelectorAll('[data-mode]').forEach((x) => x.classList.toggle('active', x === b));
    box.querySelector('#mode-tip').textContent = m.tip || '';
    box.querySelector('#score-label').textContent = `First to (${m.scoreLabel})`;
    const score = box.querySelector('[name=scoreLimit]'); score.value = m.score; score.max = m.maxScore;
    box.querySelector('#time-field').hidden = !m.time;
    if (m.time) box.querySelector('[name=timeLimit]').value = m.time;
    refresh();
  }));
  box.addEventListener('input', refresh);
  box.addEventListener('change', refresh);
  return read;
}
