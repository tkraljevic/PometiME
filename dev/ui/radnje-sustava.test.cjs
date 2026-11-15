const { test } = require('node:test');
const assert = require('node:assert/strict');
const vm = require('node:vm');
const { html, prijevod } = require('./prijevod.cjs');

const pomocne = html.slice(html.indexOf('function formatirajBajte('), html.indexOf('// Siguran Markdown parser'));
const radnje = html.slice(html.indexOf('// RADNJE SUSTAVA:'), html.indexOf('// DIJALOG NADOGRADNJE'));
const karticeSustava = html.slice(html.indexOf('function iscrtajAlatSustav('), html.indexOf('// Sekcija pokretača sustava'));

// Ove radnje brišu odmah, bez Koša.
const TRAJNE = ['snapshots', 'aerials', 'simulators'];

function kontekst() {
  const zahtjevi = [];
  const dijalozi = [];
  const context = vm.createContext({
    document: { documentElement: { lang: 'hr' } },
    stanje: { snimke: ['a', 'b', 'c'], aerials: { total_bytes: 2048, video_count: 2 } },
    otvoriDijalogPotvrde: (naslov, opis, gumb, potvrdi) => dijalozi.push({ naslov, opis, gumb, potvrdi }),
    apiFetch: async (adresa, opcije) => {
      zahtjevi.push({ adresa, tijelo: JSON.parse(opcije.body) });
      return { json: async () => ({ message: 'u redu' }) };
    },
    prikaziPoruku: () => {}
  });
  vm.runInContext(prijevod + pomocne + radnje, context);
  return { context, zahtjevi, dijalozi };
}

for (const akcija of TRAJNE) {
  test(`${akcija}: ništa se ne briše dok korisnik ne potvrdi`, async () => {
    const k = kontekst();
    let osvjezeno = 0;
    k.context.zatraziPotvrduRadnjeSustava(akcija, async () => { osvjezeno++; });

    assert.equal(k.dijalozi.length, 1);
    assert.deepEqual(k.zahtjevi, [], 'zahtjev ne smije otići prije potvrde');
    assert.match(k.dijalozi[0].opis, /ne ide u Koš i ne može se vratiti/);

    await k.dijalozi[0].potvrdi();
    assert.deepEqual(k.zahtjevi, [{ adresa: '/api/optimiziraj', tijelo: { action: akcija } }]);
    assert.equal(osvjezeno, 1);
  });
}

test('potvrda kaže koliko će se obrisati', () => {
  const k = kontekst();
  k.context.zatraziPotvrduRadnjeSustava('snapshots', async () => {});
  k.context.zatraziPotvrduRadnjeSustava('aerials', async () => {});
  assert.match(k.dijalozi[0].opis, /3 lokalne snimke/);
  assert.match(k.dijalozi[1].opis, /2 videozapisa \(2,0 KB\)/);
});

test('gumbi na karticama sustava ne brišu bez potvrde', () => {
  for (const akcija of TRAJNE) {
    assert.doesNotMatch(karticeSustava, new RegExp(`pokreniOptimizaciju\\('${akcija}'\\)`), `${akcija} se pokreće bez potvrde`);
    assert.match(karticeSustava, new RegExp(`zatraziPotvrduRadnjeSustava\\('${akcija}'`));
  }
});
