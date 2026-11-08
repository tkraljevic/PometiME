const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');
const html = fs.readFileSync(path.join(__dirname, '../../internal/web/sucelje.html'), 'utf8');
const source = html.slice(html.indexOf('function postaviStanjeNadogradnje('), html.indexOf('// Učitavanje podataka s poslužitelja'));

function ui(data, installStatus = 200, restart = null) {
  const elements = new Map();
  const element = id => {
    if (!elements.has(id)) elements.set(id, { style: {}, classList: { add() {}, remove() {} }, showModal() { this.open = true; } });
    return elements.get(id);
  };
  const calls = [];
  const context = vm.createContext({
    document: { getElementById: element },
    stanje: {},
    window: { webkit: restart ? { messageHandlers: { pometimeRestart: { postMessage: restart } } } : undefined },
    renderirajMarkdown: s => s,
    fetch: async (url, options) => {
      calls.push({ url, options });
      return { ok: !options || installStatus === 200, json: async () => options ? { success: installStatus === 200, error: 'Nema dozvole za zamjenu.' } : data };
    }
  });
  vm.runInContext(source, context);
  return { context, element, calls };
}
const release = {
  current_version: '1.0.3', latest_version: '1.0.6', has_update: true,
  release_notes: 'Ispravak nadogradnje.', html_url: 'https://github.com/tkraljevic/PometiME/releases/tag/v1.0.6',
  download_url: 'https://github.com/tkraljevic/PometiME/releases/download/v1.0.6/PometiME-macOS.zip'
};

test('GitHub API odgovor prikazuje dostupnu verziju, bilješke i oznaku nadogradnje', async () => {
  const u = ui(release);
  await u.context.provjeriNadogradnju(true);
  assert.equal(u.element('nadogradnja-naslov').textContent, 'Nova verzija je spremna.');
  assert.equal(u.element('nadogradnja-stara-verzija').textContent, '1.0.3');
  assert.equal(u.element('nadogradnja-nova-verzija').textContent, '1.0.6');
  assert.equal(u.element('nadogradnja-biljeske-tekst').innerHTML, release.release_notes);
  assert.equal(u.element('indikator-nove-verzije').style.display, 'inline-flex');
});

test('instalacija šalje URL paketa poslužitelju i prikazuje instaliranu verziju', async () => {
  const u = ui(release);
  await u.context.provjeriNadogradnju(true);
  await u.context.instalirajNadogradnju();
  assert.equal(u.calls[1].url, '/api/nadogradnja/primijeni');
  assert.equal(JSON.parse(u.calls[1].options.body).download_url, release.download_url);
  assert.equal(u.element('nadogradnja-naslov').textContent, 'Nadogradnja je instalirana.');
  assert.equal(u.element('nadogradnja-status-poruka').textContent, 'Instalirana verzija 1.0.6');
});

test('ažurna instalacija prikazuje stvarnu verziju API-ja', async () => {
  const u = ui({ current_version: '2.4.1', latest_version: '2.4.1', has_update: false });
  await u.context.provjeriNadogradnju(true);
  assert.equal(u.element('nadogradnja-naslov').textContent, 'Sve je ažurno.');
  assert.equal(u.element('nadogradnja-status-poruka').textContent, 'Instalirana verzija 2.4.1');
  assert.equal(u.element('indikator-nove-verzije').style.display, 'none');
});

test('greška instalacije ostaje vidljiva umjesto prikaza uspjeha', async () => {
  const u = ui(release, 500);
  await u.context.provjeriNadogradnju(true);
  await u.context.instalirajNadogradnju();
  assert.equal(u.element('nadogradnja-naslov').textContent, 'Instalacija nije uspjela.');
  assert.equal(u.element('nadogradnja-status-poruka').textContent, 'Nema dozvole za zamjenu.');
});

test('gumb za ponovno pokretanje pojavljuje se tek nakon instalacije u nativnoj aplikaciji', () => {
  const u = ui(release, 200, async () => true);
  for (const state of ['Checking', 'Available', 'Installing', 'Current', 'InstallError']) {
    u.context.postaviStanjeNadogradnje(state, release);
    assert.equal(u.element('nadogradnja-gumb-restart').style.display, 'none');
  }
  u.context.postaviStanjeNadogradnje('Installed', release);
  assert.equal(u.element('nadogradnja-gumb-restart').style.display, 'inline-flex');
});

test('bez nativnog mosta ostaje uputa za ručno ponovno pokretanje', () => {
  const u = ui(release);
  u.context.postaviStanjeNadogradnje('Installed', release);
  assert.equal(u.element('nadogradnja-gumb-restart').style.display, 'none');
  assert.match(u.element('nadogradnja-podnaslov').textContent, /ponovno/);
});

test('ponovno pokretanje zahtijeva instalaciju i dvostruki klik šalje samo jedan zahtjev', async () => {
  let calls = 0, finish;
  const u = ui(release, 200, () => { calls++; return new Promise(resolve => { finish = resolve; }); });
  await u.context.ponovnoPokreniPometiME();
  assert.equal(calls, 0);
  u.context.postaviStanjeNadogradnje('Installed', release);
  const first = u.context.ponovnoPokreniPometiME();
  await u.context.ponovnoPokreniPometiME();
  assert.equal(calls, 1);
  assert.equal(u.element('nadogradnja-gumb-restart').disabled, true);
  finish(true); await first;
});

test('neuspjelo ponovno pokretanje vraća gumbe i pokazuje ručnu uputu', async () => {
  const u = ui(release, 200, async () => { throw new Error('Helper nije pokrenut'); });
  u.context.postaviStanjeNadogradnje('Installed', release);
  await u.context.ponovnoPokreniPometiME();
  assert.equal(u.element('nadogradnja-gumb-restart').disabled, false);
  assert.equal(u.element('nadogradnja-gumb-zatvori').disabled, false);
  assert.match(u.element('nadogradnja-status-poruka').textContent, /⌘Q/);
});
