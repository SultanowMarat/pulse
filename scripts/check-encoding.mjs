import fs from 'node:fs';
import path from 'node:path';
import { execSync } from 'node:child_process';

const roots = ['web/src', 'web/public/sw.js', 'docs'];
const exts = new Set(['.ts', '.tsx', '.js', '.jsx', '.mjs', '.json', '.css', '.html', '.md']);

const files = execSync('git ls-files', { encoding: 'utf8' })
  .split(/\r?\n/)
  .filter(Boolean)
  .filter((f) => roots.some((r) => f === r || f.startsWith(`${r}/`)))
  .filter((f) => exts.has(path.extname(f).toLowerCase()) || f.endsWith('sw.js'))
  .filter((f) => fs.existsSync(f));

const badPatterns = [
  /\u00D0[\u0080-\u00BF]/g,
  /\u00D1[\u0080-\u00BF]/g,
  /\u00C3[\u0080-\u00BF]/g,
  /\u00C2[\u0080-\u00BF]/g,
  /\uFFFD/g,
  /\u0424\{/g,
];

const issues = [];

for (const file of files) {
  const text = fs.readFileSync(file, 'utf8');
  if (badPatterns.some((re) => re.test(text))) {
    issues.push(file);
  }
}

if (issues.length > 0) {
  console.error('Encoding check failed. Suspicious mojibake found in files:');
  for (const f of issues) console.error(` - ${f}`);
  process.exit(1);
}

console.log(`Encoding check passed (${files.length} files checked).`);