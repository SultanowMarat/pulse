import sharp from 'sharp';
import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const publicDir = path.join(__dirname, '..', 'public');
const logoPngPath = path.join(publicDir, 'brand', 'buhchat.png');
const svgPath = path.join(publicDir, 'icon.svg');
const iconsDir = path.join(publicDir, 'icons');

const sourcePath = fs.existsSync(logoPngPath) ? logoPngPath : svgPath;
if (!fs.existsSync(sourcePath)) {
  console.error('Icon source not found (expected public/brand/buhchat.png or public/icon.svg)');
  process.exit(1);
}
if (!fs.existsSync(iconsDir)) {
  fs.mkdirSync(iconsDir, { recursive: true });
}

const sizes = [120, 152, 167, 180, 192, 512];
const sourceBuffer = fs.readFileSync(sourcePath);

for (const size of sizes) {
  await sharp(sourceBuffer).resize(size, size).png().toFile(path.join(iconsDir, `icon-${size}.png`));
  console.log(`Generated icon-${size}.png`);
}

// iOS Home Screen often probes these canonical root names.
await sharp(sourceBuffer).resize(180, 180).png().toFile(path.join(publicDir, 'apple-touch-icon.png'));
await sharp(sourceBuffer).resize(180, 180).png().toFile(path.join(publicDir, 'apple-touch-icon-precomposed.png'));
console.log('Generated apple-touch-icon.png');
console.log('Generated apple-touch-icon-precomposed.png');

console.log(`Icons generated in public/icons/ from ${path.relative(publicDir, sourcePath)}`);
