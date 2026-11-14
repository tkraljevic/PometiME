const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');
const { html, prijevod } = require('./prijevod.cjs');

const pomocne = html.slice(html.indexOf('function formatirajBajte('), html.indexOf('// Siguran Markdown parser'));
const uklanjanje = html.slice(html.indexOf('// Akcija uklanjanja pokretača'), html.indexOf('// Akcija povrata iz Koša'));

// Imena polja koja poslužitelj šalje za pokretač (PokretacSustava u model.go).
const model = fs.readFileSync(path.join(__dirname, '../../internal/cistac/model.go'), 'utf8');
const pocetakStrukture = model.indexOf('type PokretacSustava struct');
const poljaPokretaca = [...model.slice(pocetakStrukture, model.indexOf('}', pocetakStrukture)).matchAll(/json:"([^"]+)"/g)].map(m => m[1]);

function pokretacKakavStizeSPosluzitelja(vrijednosti) {
  for (const kljuc of Object.keys(vrijednosti)) assert.ok(poljaPokretaca.includes(kljuc), `poslužitelj ne šalje polje ${kljuc}`);
  return vrijednosti;
}

function kontekst() {
  const zahtjevi = [];
  const dijalozi = [];
  const context = vm.createContext({
    document: { documentElement: { lang: 'hr' } },
    otvoriDijalogPotvrde: (naslov, opis, gumb, potvrdi) => dijalozi.push({ opis, potvrdi }),
    apiFetch: async (adresa, opcije) => {
      zahtjevi.push({ adresa, tijelo: JSON.parse(opcije.body) });
      return { json: async () => ({ message: 'u redu' }) };
    },
    prikaziPoruku: () => {},
    ucitajPokretace: async () => {},
    ucitajKos: async () => {}
  });
  vm.runInContext(prijevod + pomocne + uklanjanje, context);
  return { context, zahtjevi, dijalozi };
}

const plist = '/Users/primjer/Library/LaunchAgents/hr.primjer.plist';

test('uklanjanje jednog pokretača šalje putanju njegove datoteke', async () => {
  const k = kontekst();
  k.context.zatraziPotvrduUklanjanjaPokretaca(pokretacKakavStizeSPosluzitelja({ name: 'hr.primjer.plist', path: plist, status: 'siroce' }));
  assert.match(k.dijalozi[0].opis, /hr\.primjer\.plist/);
  await k.dijalozi[0].potvrdi();
  assert.deepEqual(k.zahtjevi, [{ adresa: '/api/ocisti', tijelo: { paths: [plist] } }]);
});

test('uklanjanje svih zaostalih pokretača šalje sve njihove putanje', async () => {
  const k = kontekst();
  const drugi = '/Users/primjer/Library/LaunchAgents/hr.drugi.plist';
  k.context.zatraziPotvrduUklanjanjaSvihSirocadi([
    pokretacKakavStizeSPosluzitelja({ path: plist, status: 'siroce' }),
    pokretacKakavStizeSPosluzitelja({ path: drugi, status: 'siroce' })
  ]);
  await k.dijalozi[0].potvrdi();
  assert.deepEqual(k.zahtjevi[0].tijelo, { paths: [plist, drugi] });
});

test('popis pokretača čita samo polja koja poslužitelj šalje', () => {
  const crtanje = html.slice(html.indexOf('const sirocad = (stanje.pokretaci || [])'), html.indexOf('// Akcija povrata iz Koša'));
  const procitana = new Set([...crtanje.matchAll(/\b[ps]\.([a-z_]+)\b/g)].map(m => m[1]));
  assert.ok(procitana.size >= 4, 'test mora pronaći polja koja sučelje čita');
  assert.deepEqual([...procitana].filter(polje => !poljaPokretaca.includes(polje)), []);
});
