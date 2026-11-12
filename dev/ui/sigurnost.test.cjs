const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');
const html = fs.readFileSync(path.join(__dirname, '../../internal/web/sucelje.html'), 'utf8');
const izmedu = (pocetak, kraj) => html.slice(html.indexOf(pocetak), html.indexOf(kraj));
const zastita = izmedu('// Ključ sesije', '// Stanje sučelja');
const stablo = izmedu('function iscrtajStabloHtml(', 'function prikaziStabloPopover(');

function kontekst(search) {
  const pozivi = [];
  const context = vm.createContext({
    URLSearchParams,
    window: { location: { search } },
    fetch: async (url, opcije) => { pozivi.push({ url, opcije }); return { ok: true }; },
    formatirajBajte: b => b + ' B'
  });
  vm.runInContext(zastita + stablo, context);
  return { context, pozivi };
}

test('svaki API poziv nosi ključ sesije iz adrese i zadržava ostala zaglavlja', async () => {
  const k = kontekst('?k=ABC123');
  await k.context.apiFetch('/api/ocisti', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}' });
  await k.context.apiFetch('/api/kos');
  assert.equal(k.pozivi[0].opcije.headers['X-PometiME-Kljuc'], 'ABC123');
  assert.equal(k.pozivi[0].opcije.headers['Content-Type'], 'application/json');
  assert.equal(k.pozivi[0].opcije.method, 'POST');
  assert.equal(k.pozivi[1].opcije.headers['X-PometiME-Kljuc'], 'ABC123');
});

test('bez ključa u adresi šalje se prazan ključ, pa poslužitelj odbija zahtjev', async () => {
  const k = kontekst('');
  await k.context.apiFetch('/api/kos');
  assert.equal(k.pozivi[0].opcije.headers['X-PometiME-Kljuc'], '');
});

test('ime datoteke s HTML oznakama prikazuje se kao tekst', () => {
  const k = kontekst('');
  const zlo = '<img src=x onerror="alert(1)">.dmg';
  const izlaz = k.context.iscrtajStabloHtml([{ name: zlo, path: '/Users/x/"' + zlo, size_bytes: 1, is_dir: false }]);
  assert.ok(!izlaz.includes('<img'), izlaz);
  assert.ok(izlaz.includes('&lt;img src=x onerror=&quot;alert(1)&quot;&gt;.dmg'), izlaz);
  assert.equal(k.context.escapirajHtml(null), '');
  assert.equal(k.context.escapirajHtml("a'b&c"), 'a&#39;b&amp;c');
});
