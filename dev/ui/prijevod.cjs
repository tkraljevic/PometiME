// Blok prijevoda iz sucelje.html (t, PRIJEVODI_EN, PLURALI_EN) za testove koji izrezuju dijelove skripte.
const fs = require('node:fs');
const path = require('node:path');

const html = fs.readFileSync(path.join(__dirname, '../../internal/web/sucelje.html'), 'utf8');
const prijevod = html.slice(html.indexOf('// PRIJEVOD: početak'), html.indexOf('// PRIJEVOD: kraj'));

module.exports = { html, prijevod };
