import sharp from 'sharp';
import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.join(__dirname, '..', '..');
const publicDir = path.join(__dirname, '..', 'public');
const externalPackDir = process.env.PULSE_ICON_PACK_DIR
  ? path.resolve(process.env.PULSE_ICON_PACK_DIR)
  : path.join(repoRoot, 'pulse_icon_pack');
const externalLogoPngPath = path.join(externalPackDir, 'pulse_1024.png');
const logoPngPath = path.join(publicDir, 'brand', 'pulse.png');
const svgPath = path.join(publicDir, 'icon.svg');
const iconsDir = path.join(publicDir, 'icons');

if (!fs.existsSync(path.join(publicDir, 'brand'))) {
  fs.mkdirSync(path.join(publicDir, 'brand'), { recursive: true });
}
if (fs.existsSync(externalLogoPngPath)) {
  fs.copyFileSync(externalLogoPngPath, logoPngPath);
}

const sourcePath = fs.existsSync(logoPngPath) ? logoPngPath : svgPath;
if (!fs.existsSync(sourcePath)) {
  console.error('Icon source not found (expected pulse_icon_pack/pulse_1024.png, public/brand/pulse.png or public/icon.svg)');
  process.exit(1);
}
if (!fs.existsSync(iconsDir)) {
  fs.mkdirSync(iconsDir, { recursive: true });
}

const sizes = [120, 152, 167, 180, 192, 512];
const sourceBuffer = fs.readFileSync(sourcePath);

const renderSquarePng = (size, outputPath) =>
  sharp(sourceBuffer)
    .resize(size, size, {
      fit: 'contain',
      background: { r: 0, g: 0, b: 0, alpha: 0 },
    })
    .png()
    .toFile(outputPath);

for (const size of sizes) {
  await renderSquarePng(size, path.join(iconsDir, `icon-${size}.png`));
  console.log(`Generated icon-${size}.png`);
}

// iOS Home Screen often probes these canonical root names.
await renderSquarePng(180, path.join(publicDir, 'apple-touch-icon.png'));
await renderSquarePng(180, path.join(publicDir, 'apple-touch-icon-precomposed.png'));
console.log('Generated apple-touch-icon.png');
console.log('Generated apple-touch-icon-precomposed.png');
await renderSquarePng(1024, logoPngPath);
console.log('Generated pulse brand logo');

console.log(`Icons generated in public/icons/ from ${path.relative(publicDir, sourcePath)}`);
