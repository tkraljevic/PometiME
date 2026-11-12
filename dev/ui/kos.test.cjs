const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');
const html = fs.readFileSync(path.join(__dirname, '../../internal/web/sucelje.html'), 'utf8');
const source = html.slice(html.indexOf('// Stavka u Košu ovoliko dana'), html.indexOf('function iscrtajKos('));

function kontekst(kos) {
  const naslov = { textContent: '' };
  const opis = { textContent: '' };
  const kartica = { querySelector: sel => (sel.includes('title') ? naslov : opis) };
  const context = vm.createContext({
    stanje: { kos },
    document: { getElementById: id => (id === 'brza-akcija-kos' ? kartica : null) },
    formatirajBajte: b => b + ' B'
  });
  vm.runInContext(source, context);
  return { context, naslov, opis };
}

test('Pregled kaže koliko prostora čeka u Košu', () => {
  const k = kontekst([{ size_bytes: 300, days_in_trash: 1 }, { size_bytes: 700, days_in_trash: 9 }]);
  k.context.osvjeziKarticuKosa();
  assert.match(k.naslov.textContent, /U Košu čeka 1000 B/);
  assert.match(k.opis.textContent, /trajno izbrišete/);
});

test('prazan Koš vraća uobičajenu karticu za povrat', () => {
  const k = kontekst([]);
  k.context.osvjeziKarticuKosa();
  assert.match(k.naslov.textContent, /Vratite očišćene stavke/);
});

test('brzi odabir uzima samo stavke starije od 7 dana', () => {
  const k = kontekst([
    { trash_path: '/a', days_in_trash: 6 },
    { trash_path: '/b', days_in_trash: 7 },
    { trash_path: '/c', days_in_trash: 30 },
    { trash_path: '/d' }
  ]);
  assert.deepEqual(k.context.stareStavkeKosa().map(st => st.trash_path), ['/b', '/c']);
});
