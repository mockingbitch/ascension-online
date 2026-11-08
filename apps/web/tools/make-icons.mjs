// Generate the PWA icons from the existing logo (static/favicon.svg — the gold/ink taiji mark): no new art, no
// dependencies (a tiny PNG encoder over node:zlib). The mark's geometry is drawn analytically with 4×4 supersampling;
// colours are read from favicon.svg so the icons follow the logo. Run after changing the logo:
//   node apps/web/tools/make-icons.mjs
// Output (committed): static/icons/icon-192.png, icon-512.png (the round mark on a transparent corner),
// icon-maskable-512.png (full-bleed ink background, mark inside the 80 % safe zone), apple-touch-icon.png (180).
// The maskable background is the --color-ink-950 token (literal here: a PNG cannot read CSS variables).
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const svg = fs.readFileSync(path.join(root, 'static/favicon.svg'), 'utf8');
// guard: this renderer mirrors the favicon's shapes; if the logo changes shape, update draw() below
const expect = ['circle cx="32" cy="32" r="30"', 'M32 6a26 26 0 0 0 0 52 13 13 0 0 1 0-26 13 13 0 0 0 0-26z', 'cx="32" cy="19" r="4"', 'cx="32" cy="45" r="4"'];
for (const e of expect) if (!svg.includes(e)) throw new Error(`favicon.svg changed (missing ${e}); update tools/make-icons.mjs`);
const fills = [...svg.matchAll(/fill="(#[0-9a-fA-F]{6})"/g)].map((m) => m[1]);
const [INK, GOLD] = [fills[0], fills[1]];
const INK_950 = '#0a0f10'; // --color-ink-950 (app.css)

const hex = (h) => [parseInt(h.slice(1, 3), 16), parseInt(h.slice(3, 5), 16), parseInt(h.slice(5, 7), 16)];
const inC = (x, y, cx, cy, r) => (x - cx) ** 2 + (y - cy) ** 2 <= r * r;

/** Colour of the 64×64 logo at (x, y): null = outside the mark. */
function draw(x, y) {
	if (!inC(x, y, 32, 32, 30)) return null;
	if (inC(x, y, 32, 19, 4)) return INK;
	if (inC(x, y, 32, 45, 4)) return GOLD;
	// the path: left half of the r26 disc, minus the lower r13 half-disc, plus the upper r13 half-disc
	const gold = inC(x, y, 32, 19, 13) || (inC(x, y, 32, 32, 26) && x < 32 && !inC(x, y, 32, 45, 13));
	return gold ? GOLD : INK;
}

function render(size, { bg = null, scale = 1 } = {}) {
	const px = Buffer.alloc(size * size * 4);
	const S = 4;
	const bgc = bg ? hex(bg) : null;
	for (let j = 0; j < size; j++)
		for (let i = 0; i < size; i++) {
			let r = 0, g = 0, b = 0, a = 0;
			for (let sj = 0; sj < S; sj++)
				for (let si = 0; si < S; si++) {
					// logo space: centre the 64-unit mark, scaled
					const u = ((i + (si + 0.5) / S) / size - 0.5) / scale * 64 + 32;
					const v = ((j + (sj + 0.5) / S) / size - 0.5) / scale * 64 + 32;
					const c = draw(u, v);
					const rgb = c ? hex(c) : bgc;
					if (!rgb) continue;
					r += rgb[0]; g += rgb[1]; b += rgb[2]; a += 1;
				}
			const o = (j * size + i) * 4;
			const n = S * S;
			px[o] = a ? Math.round(r / a) : 0;
			px[o + 1] = a ? Math.round(g / a) : 0;
			px[o + 2] = a ? Math.round(b / a) : 0;
			px[o + 3] = Math.round((a / n) * 255);
		}
	return png(size, size, px);
}

const crcTable = Array.from({ length: 256 }, (_, n) => {
	let c = n;
	for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
	return c >>> 0;
});
const crc32 = (buf) => {
	let c = 0xffffffff;
	for (const b of buf) c = crcTable[(c ^ b) & 0xff] ^ (c >>> 8);
	return (c ^ 0xffffffff) >>> 0;
};
function chunk(type, data) {
	const len = Buffer.alloc(4);
	len.writeUInt32BE(data.length);
	const td = Buffer.concat([Buffer.from(type), data]);
	const crc = Buffer.alloc(4);
	crc.writeUInt32BE(crc32(td));
	return Buffer.concat([len, td, crc]);
}
function png(w, h, rgba) {
	const ihdr = Buffer.alloc(13);
	ihdr.writeUInt32BE(w, 0);
	ihdr.writeUInt32BE(h, 4);
	ihdr[8] = 8; // bit depth
	ihdr[9] = 6; // RGBA
	const raw = Buffer.alloc((w * 4 + 1) * h);
	for (let y = 0; y < h; y++) rgba.copy(raw, y * (w * 4 + 1) + 1, y * w * 4, (y + 1) * w * 4);
	return Buffer.concat([Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]), chunk('IHDR', ihdr), chunk('IDAT', zlib.deflateSync(raw, { level: 9 })), chunk('IEND', Buffer.alloc(0))]);
}

const out = path.join(root, 'static/icons');
fs.mkdirSync(out, { recursive: true });
const write = (name, buf) => {
	fs.writeFileSync(path.join(out, name), buf);
	console.log(`static/icons/${name} ${(buf.length / 1024).toFixed(1)} kB`);
};
write('icon-192.png', render(192));
write('icon-512.png', render(512));
// maskable: the mark within the inner 80 % circle (safe zone), ink background to the edges
write('icon-maskable-512.png', render(512, { bg: INK_950, scale: 0.72 }));
write('apple-touch-icon.png', render(180, { bg: INK_950, scale: 0.84 }));
