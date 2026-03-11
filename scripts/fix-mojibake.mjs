import fs from 'node:fs';
import path from 'node:path';
import { execSync } from 'node:child_process';

const exts = new Set([
  '.ts', '.tsx', '.js', '.jsx', '.mjs', '.cjs', '.json', '.css', '.html', '.md', '.txt', '.go',
  '.yml', '.yaml', '.conf', '.sh', '.env', '.ini', '.toml', '.sql', '.xml'
]);

const re = /(?:[ÐÑÃÂ][\u0080-\u00FF])+/g;

function decodeIter(s) {
  let cur = s;
  for (let i = 0; i < 3; i++) {
    const next = Buffer.from(cur, 'latin1').toString('utf8');
    if (next === cur) break;
    cur = next;
  }
  return cur;
}

const files = execSync('git ls-files', { encoding: 'utf8' })
  .split(/\r?\n/)
  .filter(Boolean)
  .filter((f) => exts.has(path.extname(f).toLowerCase()))
  .filter((f) => !f.startsWith('web/node_modules/') && !f.startsWith('dist/') && !f.includes('/dist/'));

let changedFiles = 0;
let replaced = 0;

for (const file of files) {
  let txt;
  try { txt = fs.readFileSync(file, 'utf8'); } catch { continue; }
  if (!/[ÐÑÃÂ]/.test(txt)) continue;

  let local = 0;
  const out = txt.replace(re, (m) => {
    const d = decodeIter(m);
    if (d !== m) {
      local++;
      return d;
    }
    return m;
  });

  if (out !== txt) {
    fs.writeFileSync(file, out, 'utf8');
    changedFiles++;
    replaced += local;
  }
}

console.log(`changed_files=${changedFiles}`);
console.log(`replaced_chunks=${replaced}`);
