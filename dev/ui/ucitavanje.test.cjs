const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');
const html = fs.readFileSync(path.join(__dirname, '../../internal/web/sucelje.html'), 'utf8');
const source = html.slice(html.indexOf('// Učitavanje popisa:'), html.indexOf('// Formatiranje bajtova'));
function setup(fetcher) {
  let now = 0, next = 0, renders = 0;
  const timers = new Map();
  const context = vm.createContext({
    stanje: {}, apiFetch: fetcher,
    setTimeout: (fn, delay) => { timers.set(++next, { fn, time: now + delay }); return next; },
    clearTimeout: id => timers.delete(id),
    iscrtajAplikacije: () => renders++, iscrtajCiscenje() {}, iscrtajNapredneAlate() {}, iscrtajKos() {},
    ucitajAplikacije() {}, ucitajKos() {}, ucitajSkeniranje() {}, postaviBadge() {}, osvjeziDonjuTraku() {}
  });
  vm.runInContext(source, context);
  return {
    context,
    status: () => vm.runInContext('ucitavanja.aplikacije', context),
    timers,
    renders: () => renders,
    advance(ms) {
      now += ms;
      for (const [id, timer] of timers) if (timer.time <= now) { timers.delete(id); timer.fn(); }
    }
  };
}
test('loader se ne pojavljuje prije 180 ms i nakon 8 s prepoznaje dulje učitavanje', () => {
  const u = setup();
  u.context.zapocniUcitavanje('aplikacije');
  u.advance(179); assert.equal(u.status().vidljivo, false);
  u.advance(1); assert.equal(u.status().vidljivo, true);
  u.advance(7820); assert.equal(u.status().dulje, true);
});
test('brz odgovor uklanja tajmere i ne ostavlja zakašnjeli loader', async () => {
  const u = setup(async () => ({ ok: true, json: async () => [] }));
  await u.context.ucitajPopis('aplikacije', '/api/aplikacije', 'aplikacije', 'badge');
  assert.equal(u.status().aktivno, false);
  assert.equal(u.status().vidljivo, false);
  assert.equal(u.status().greska, '');
  assert.equal(u.timers.size, 0);
  const count = u.renders(); u.advance(10000); assert.equal(u.renders(), count);
});
test('zahtjev u tijeku dijeli se pri ponovnom pozivu i zadržava dobiveni popis', async () => {
  let finish, calls = 0;
  const u = setup(() => { calls++; return new Promise(resolve => { finish = resolve; }); });
  const first = u.context.ucitajPopis('aplikacije', '/api/aplikacije', 'aplikacije', 'badge');
  const second = u.context.ucitajPopis('aplikacije', '/api/aplikacije', 'aplikacije', 'badge');
  assert.equal(first, second); assert.equal(calls, 1);
  u.advance(200); assert.equal(u.status().vidljivo, true);
  finish({ ok: true, json: async () => [{ name: 'Word' }] }); await first;
  assert.equal(u.status().aktivno, false);
  assert.equal(u.context.stanje.aplikacije[0].name, 'Word');
});
test('HTTP greška ne postaje prazan popis, a ponovni pokušaj uklanja grešku', async () => {
  let fail = true;
  const u = setup(async () => ({ ok: !fail, json: async () => [] }));
  await u.context.ucitajPopis('aplikacije', '/api/aplikacije', 'aplikacije', 'badge');
  assert.match(u.status().greska, /Dohvat/);
  assert.equal(u.context.stanje.aplikacije, undefined);
  fail = false;
  await u.context.ucitajPopis('aplikacije', '/api/aplikacije', 'aplikacije', 'badge');
  assert.equal(u.status().greska, ''); assert.equal(u.status().aktivno, false);
});
test('neispravan API odgovor prikazuje grešku i zaustavlja animaciju', async () => {
  const u = setup(async () => ({ ok: true, json: async () => ({ error: 'not a list' }) }));
  await u.context.ucitajPopis('aplikacije', '/api/aplikacije', 'aplikacije', 'badge');
  assert.match(u.status().greska, /neispravan popis/);
  assert.equal(u.status().aktivno, false); assert.equal(u.timers.size, 0);
});
