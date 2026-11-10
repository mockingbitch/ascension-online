// Fails the build when art files are Git LFS pointers instead of images.
// A checkout without git-lfs leaves ~130-byte text pointers under src/lib/assets/art; Vite would bundle them as
// broken .webp/.png files and every painted image would silently disappear in the game.
import fs from 'node:fs';
import path from 'node:path';

const root = path.resolve(import.meta.dirname, '../src/lib/assets/art');
const POINTER = 'version https://git-lfs.github.com/spec/v1';
const bad = [];

function walk(dir) {
	for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
		const p = path.join(dir, e.name);
		if (e.isDirectory()) walk(p);
		else if (/\.(webp|png|avif|jpg)$/i.test(e.name) && fs.statSync(p).size < 1024) {
			if (fs.readFileSync(p, 'utf8').startsWith(POINTER)) bad.push(path.relative(root, p));
		}
	}
}

if (fs.existsSync(root)) walk(root);
if (bad.length) {
	console.error(
		`\n✗ ${bad.length} art file(s) are Git LFS pointers, not images (e.g. ${bad.slice(0, 3).join(', ')}).\n` +
			'  Install Git LFS and fetch the images before building:\n' +
			'    git lfs install && git lfs pull\n'
	);
	process.exit(1);
}
