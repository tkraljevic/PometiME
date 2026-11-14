const { test } = require('node:test');
const assert = require('node:assert/strict');
const vm = require('node:vm');
const { html, prijevod } = require('./prijevod.cjs');

const skripta = html.slice(html.indexOf('<script>'), html.indexOf('</script>'));
const pomocne = html.slice(html.indexOf('function formatirajBajte('), html.indexOf('// Siguran Markdown parser'));

function kontekst(lang) {
  const context = vm.createContext({ document: { documentElement: { lang } } });
  vm.runInContext(prijevod + pomocne + '\nglobalThis.RJECNIK = PRIJEVODI_EN; globalThis.MNOZINE = PLURALI_EN;', context);
  return context;
}

// Vrijednost JS niza iz izvora, s escape-nizovima kakve vidi preglednik.
const vrijednost = sirovo => vm.runInNewContext(`'${sirovo}'`);

function bezUmetaka(predlozak) {
  let izlaz = '';
  for (let i = 0; i < predlozak.length; i++) {
    if (predlozak.startsWith('${', i)) {
      let dubina = 1;
      for (i += 2; i < predlozak.length && dubina; i++) dubina += predlozak[i] === '{' ? 1 : predlozak[i] === '}' ? -1 : 0;
      i--;
      continue;
    }
    izlaz += predlozak[i];
  }
  return izlaz;
}

const tekstoviIzHtmla = dio => [...dio.matchAll(/>([^<>]+)</g)]
  .map(m => m[1].replace(/&rarr;/g, '→').replace(/&ldquo;/g, '“').replace(/&rdquo;/g, '”')
    .replace(/&larr;/g, '←').replace(/&check;/g, '✓').replace(/&middot;/g, '·').replace(/&nearr;/g, '↗')
    .replace(/&amp;/g, '&').replace(/\s+/g, ' ').trim())
  .filter(tekst => /[A-Za-zčćžšđ]{2}/.test(tekst.replace(/&[a-z]+;/g, '')) && !/^PometiME( [\d.]+)?$/.test(tekst) && !['Macintosh HD', 'HR', 'EN'].includes(tekst));

test('na hrvatskom tekst ostaje izvorni, a brojevi imaju decimalni zarez', () => {
  const k = kontekst('hr');
  assert.equal(k.t('Odabrano {n}', { n: k.uzBrojHR(3, 'stavka', 'stavke', 'stavki') }), 'Odabrano 3 stavke');
  assert.equal(k.formatirajBajte(1536), '1,5 KB');
});

test('na engleskom t prevodi, umeće vrijednosti i ostavlja nepoznat tekst', () => {
  const k = kontekst('en');
  assert.equal(k.t('Odabrano {n}', { n: k.uzBrojHR(1, 'stavka', 'stavke', 'stavki') }), '1 item selected');
  assert.equal(k.uzBrojHR(3, 'stavka', 'stavke', 'stavki'), '3 items');
  assert.equal(k.uzBrojHR(2, 'nepoznata riječ', 'nepoznate riječi', 'nepoznatih riječi'), '2 nepoznate riječi');
  assert.equal(k.t('Tekst bez prijevoda'), 'Tekst bez prijevoda');
  assert.equal(k.t('Izmjereno u {vrijeme}'), 'Measured at {vrijeme}');
  assert.equal(k.formatirajBajte(1536), '1.5 KB');
});

test('svaki tekst u sučelju ima engleski prijevod s istim oznakama', () => {
  const rjecnik = kontekst('en').RJECNIK;
  const potrebni = new Set();
  for (const m of skripta.matchAll(/\bt\(\s*((?:'(?:[^'\\]|\\.)*'\s*(?:\+\s*)?)+)\s*[,)]/g)) {
    potrebni.add([...m[1].matchAll(/'((?:[^'\\]|\\.)*)'/g)].map(d => vrijednost(d[1])).join(''));
  }
  const bezSkripte = html.replace(/<script>[\s\S]*<\/script>/, '').replace(/<style>[\s\S]*<\/style>/, '');
  tekstoviIzHtmla(bezSkripte).forEach(tekst => potrebni.add(tekst));
  for (const m of bezSkripte.matchAll(/(?:placeholder|title|aria-label)="([^"]*[A-Za-zčćžšđ]{2}[^"]*)"/g)) potrebni.add(m[1]);
  for (const m of skripta.matchAll(/innerHTML = `([^`]*)`/g)) tekstoviIzHtmla(bezUmetaka(m[1])).forEach(tekst => potrebni.add(tekst));

  const bezPrijevoda = [...potrebni].filter(tekst => !(tekst in rjecnik));
  assert.deepEqual(bezPrijevoda, []);
  const oznake = tekst => [...tekst.matchAll(/\{(\w+)\}/g)].map(m => m[1]).sort().join(',');
  const kriveOznake = Object.entries(rjecnik).filter(([hr, en]) => oznake(hr) !== oznake(en)).map(([hr]) => hr);
  assert.deepEqual(kriveOznake, []);
  const visak = Object.keys(rjecnik).filter(tekst => !potrebni.has(tekst));
  assert.deepEqual(visak, []);
});

test('svaka riječ koju uzBrojHR dobije ima englesku jedninu i množinu', () => {
  const mnozine = kontekst('en').MNOZINE;
  const rijeci = [...skripta.matchAll(/uzBrojHR\([^,]*,\s*'([^']*)'/g)].map(m => m[1]);
  assert.deepEqual(rijeci.filter(r => !(r in mnozine)), []);
});
