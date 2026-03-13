import fs from 'node:fs';
import path from 'node:path';
import { execSync } from 'node:child_process';

const targets = execSync('git ls-files web/src web/public/sw.js', { encoding: 'utf8' })
  .split(/\r?\n/)
  .filter(Boolean)
  .filter((f) => fs.existsSync(f));

const mojibakeRe = /(?:[����][\u0080-\u00FF])+/g;

const safeAscii = new Set([
  0x09, 0x0a, 0x0d, 0x20,
  0x21, 0x22, 0x27, 0x28, 0x29, 0x2c, 0x2d, 0x2e, 0x2f,
  0x3a, 0x3f, 0x5b, 0x5d, 0x5f, 0x60, 0x7b, 0x7d
]);

function decodeMojibakeOnce(s) {
  return s.replace(mojibakeRe, (m) => Buffer.from(m, 'latin1').toString('utf8'));
}

function lowByteRestore(s) {
  let out = '';
  for (const ch of s) {
    const c = ch.charCodeAt(0);
    if (c <= 0x51 && !safeAscii.has(c)) {
      out += String.fromCharCode(0x0400 + c);
    } else {
      out += ch;
    }
  }
  return out;
}

function shouldLowByteRestore(s) {
  if (/[\u0400-\u04FF]/.test(s)) return false;
  if (/[\x00-\x1F]/.test(s)) return true;
  const suspicious = (s.match(/[><;=0-9A-OQ]/g) || []).length;
  return suspicious >= 3 && /\s/.test(s);
}

let changed = 0;
for (const file of targets) {
  const src = fs.readFileSync(file, 'utf8');
  let out = decodeMojibakeOnce(src);

  out = out.replace(/(["'`])((?:\\.|(?!\1)[\s\S])*)\1/g, (m, q, body) => {
    let b = body;
    if (/[����]/.test(b)) b = decodeMojibakeOnce(b);
    if (shouldLowByteRestore(b)) {
      const restored = lowByteRestore(b);
      if (/[\u0400-\u04FF]/.test(restored)) b = restored;
    }
    return `${q}${b}${q}`;
  });

  if (out !== src) {
    fs.writeFileSync(file, out, 'utf8');
    changed++;
  }
}

console.log(`changed_files=${changed}`);
