// Saves the realm achievement card as a PNG (lazy chunk, loaded only when the player presses "Lưu ảnh"). Canvas 2D,
// no library. It draws ONLY the CardModel (realmCard.ts: đạo hiệu, realm, stage, date, rank, title, faction, seal) and
// the card's figure — never an email, an id or anything else personal. Colours are the design tokens, read at runtime.
import type { CardModel } from './realmCard';

const W = 1080;
const H = 1350;

function token(name: string, fallback: string): string {
	const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
	return v || fallback;
}

/** An inline SVG with every CSS colour (var(), color-mix) resolved, so it renders the same as an image. */
function svgImage(svg: SVGSVGElement): Promise<HTMLImageElement> {
	const clone = svg.cloneNode(true) as SVGSVGElement;
	const src = [svg, ...svg.querySelectorAll('*')];
	const dst = [clone, ...clone.querySelectorAll('*')];
	src.forEach((el, i) => {
		const cs = getComputedStyle(el);
		const d = dst[i] as SVGElement;
		for (const p of ['fill', 'stroke', 'stop-color', 'opacity', 'stroke-opacity', 'fill-opacity', 'stop-opacity'] as const) {
			const v = cs.getPropertyValue(p);
			if (v) d.style.setProperty(p, v);
		}
		d.style.removeProperty('animation');
	});
	const r = svg.getBoundingClientRect();
	clone.setAttribute('width', String(Math.round(r.width * 2)));
	clone.setAttribute('height', String(Math.round(r.height * 2)));
	clone.setAttribute('xmlns', 'http://www.w3.org/2000/svg');
	const url = URL.createObjectURL(new Blob([new XMLSerializer().serializeToString(clone)], { type: 'image/svg+xml' }));
	return new Promise((ok, fail) => {
		const img = new Image();
		img.onload = () => ok(img);
		img.onerror = fail;
		img.src = url;
	});
}

function figureImage(el: Element | null): Promise<HTMLImageElement | null> {
	if (!el) return Promise.resolve(null);
	if (el instanceof HTMLImageElement) return el.complete ? Promise.resolve(el) : new Promise((ok) => (el.onload = () => ok(el)));
	const svg = el instanceof SVGSVGElement ? el : el.querySelector('svg');
	return svg ? svgImage(svg).catch(() => null) : Promise.resolve(null);
}

/** Draw the card and hand the PNG to the browser as a download. */
export async function exportCard(m: CardModel, figureEl: Element | null, aura: string): Promise<void> {
	const c = document.createElement('canvas');
	c.width = W;
	c.height = H;
	const g = c.getContext('2d')!;
	const ink950 = token('--color-ink-950', 'black');
	const ink800 = token('--color-ink-800', 'black');
	const gold = token('--color-gold-300', 'goldenrod');
	const gold5 = token('--color-gold-500', 'goldenrod');
	const mist = token('--color-mist-100', 'white');
	const mist3 = token('--color-mist-300', 'lightgray');
	const auraC = token(`--color-aura-${aura}`, gold);

	const bg = g.createLinearGradient(0, 0, 0, H);
	bg.addColorStop(0, ink800);
	bg.addColorStop(1, ink950);
	g.fillStyle = bg;
	g.fillRect(0, 0, W, H);
	const halo = g.createRadialGradient(W / 2, 520, 40, W / 2, 520, 520);
	halo.addColorStop(0, auraC);
	halo.addColorStop(1, 'transparent');
	g.globalAlpha = 0.35 + m.fxTier * 0.05;
	g.fillStyle = halo;
	g.fillRect(0, 0, W, H);
	g.globalAlpha = 1;
	g.strokeStyle = gold5;
	g.lineWidth = 3;
	g.strokeRect(36, 36, W - 72, H - 72);

	const fig = await figureImage(figureEl);
	if (fig) {
		const box = 680;
		const ratio = fig.naturalWidth && fig.naturalHeight ? fig.naturalWidth / fig.naturalHeight : 1;
		const w = ratio >= 1 ? box : box * ratio;
		const h = ratio >= 1 ? box / ratio : box;
		g.drawImage(fig, (W - w) / 2, 140 + (box - h), w, h);
	}

	const display = getComputedStyle(document.documentElement).getPropertyValue('--font-display').trim() || 'serif';
	const sans = getComputedStyle(document.documentElement).getPropertyValue('--font-sans').trim() || 'sans-serif';
	g.textAlign = 'center';
	g.fillStyle = gold;
	g.font = `600 30px ${sans}`;
	g.fillText(m.heading.toUpperCase(), W / 2, 108);
	g.fillStyle = auraC;
	g.font = `700 104px ${display}`;
	g.fillText(m.realmName, W / 2, 930);
	g.fillStyle = mist;
	g.font = `500 46px ${display}`;
	g.fillText(m.stageName, W / 2, 996);
	g.font = `700 64px ${display}`;
	g.fillText(m.name, W / 2, 1090);
	g.fillStyle = mist3;
	g.font = `400 32px ${sans}`;
	const meta = [m.title && `「${m.title}」`, m.faction].filter(Boolean).join(' · ');
	if (meta) g.fillText(meta, W / 2, 1144);
	g.fillText([m.rank, m.date].filter(Boolean).join(' · '), W / 2, 1196);
	g.fillStyle = gold5;
	g.font = `600 28px ${display}`;
	g.fillText('Tiên Đạo Online', W / 2, H - 70);

	if (m.seal) {
		// the static "Chí Tôn" seal (decision #6): a stamp in the corner, nothing more
		g.save();
		g.translate(W - 170, H - 230);
		g.rotate(-0.12);
		g.strokeStyle = gold;
		g.lineWidth = 5;
		g.strokeRect(-62, -62, 124, 124);
		g.fillStyle = gold;
		g.font = `700 40px ${display}`;
		g.fillText('Chí', 0, -6);
		g.fillText('Tôn', 0, 40);
		g.restore();
	}

	const blob = await new Promise<Blob | null>((ok) => c.toBlob(ok, 'image/png'));
	if (!blob) throw new Error('canvas export failed');
	const a = document.createElement('a');
	a.href = URL.createObjectURL(blob);
	a.download = m.fileName;
	document.body.appendChild(a);
	a.click();
	a.remove();
	setTimeout(() => URL.revokeObjectURL(a.href), 4000);
}
