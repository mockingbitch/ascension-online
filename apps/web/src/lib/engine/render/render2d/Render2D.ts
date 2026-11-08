// 2D SceneRenderer: layered raster art (lib/game/art.ts slots) inside a fitted frame, pointer parallax by depth on the
// shared loop, camera presets as one CSS transform. DOM + CSS only (transform/opacity), no engine. Layers whose art
// file is missing are skipped so the scene never requests a 404 (scene views draw their SVG fallbacks). Depth-0
// layers sit exactly on the frame (anchors match the painting); only moving layers get the 3% parallax overscan.
import { art } from '$lib/game/art';
import type { Quality } from '../../fx/particles';
import { gameLoop, type Loop } from '../../loop';
import type { Anchor, CameraPreset, SceneDef } from '../../scene/types';
import { anchorToScreen, cameraTransform, frameRect, type Rect } from '../frame';
import type { RendererOptions, SceneRenderer, ScreenPoint } from '../types';

/** Max parallax shift per depth unit at quality full (px); lite halves it, off disables it. */
const PARALLAX_PX = 6;

export class Render2D implements SceneRenderer {
	readonly kind = '2d' as const;
	#root: HTMLElement | null = null;
	#frameEl: HTMLDivElement | null = null;
	#layers: { el: HTMLImageElement; depth: number }[] = [];
	#def: SceneDef | null = null;
	#quality: Quality = 'full';
	#camera: CameraPreset | null = null;
	#rect: Rect = { x: 0, y: 0, width: 0, height: 0 };
	#ro: ResizeObserver | null = null;
	#loop: Loop | null;
	#unsub: (() => void) | null = null;
	#listeners = new Set<() => void>();
	#paused = false;
	// parallax target / current (-1..1)
	#tx = 0;
	#ty = 0;
	#cx = 0;
	#cy = 0;

	constructor(loop?: Loop) {
		this.#loop = loop ?? null;
	}

	async mount(el: HTMLElement, def: SceneDef, opts: RendererOptions): Promise<void> {
		this.unmount();
		this.#root = el;
		this.#def = def;
		this.#quality = opts.quality;
		this.#camera = def.camera?.[0] ?? null;
		if (getComputedStyle(el).position === 'static') el.style.position = 'relative';
		el.style.overflow = 'hidden';
		const frame = document.createElement('div');
		frame.dataset.scene = def.id;
		frame.style.cssText = 'position:absolute;transform-origin:0 0;will-change:transform';
		for (const l of def.layers) {
			const a = art(l.art);
			if (!a) continue;
			const img = document.createElement('img');
			img.src = a.src;
			if (a.srcset) img.srcset = a.srcset;
			img.alt = '';
			img.draggable = false;
			img.decoding = 'async';
			img.style.cssText =
				l.depth > 0
					? 'position:absolute;inset:-3%;width:106%;height:106%;object-fit:cover;pointer-events:none'
					: 'position:absolute;inset:0;width:100%;height:100%;object-fit:cover;pointer-events:none';
			frame.append(img);
			this.#layers.push({ el: img, depth: l.depth });
		}
		el.append(frame);
		this.#frameEl = frame;
		this.#layout();
		this.#ro = new ResizeObserver(() => this.#layout());
		this.#ro.observe(el);
		el.addEventListener('pointermove', this.#onPointer);
		el.addEventListener('pointerleave', this.#onLeave);
		this.#syncLoop();
		await Promise.all(this.#layers.map((l) => l.el.decode().catch(() => undefined)));
	}

	unmount(): void {
		this.#unsub?.();
		this.#unsub = null;
		this.#ro?.disconnect();
		this.#ro = null;
		this.#root?.removeEventListener('pointermove', this.#onPointer);
		this.#root?.removeEventListener('pointerleave', this.#onLeave);
		this.#frameEl?.remove();
		this.#frameEl = null;
		this.#layers = [];
		this.#root = null;
		this.#def = null;
	}

	anchorToScreen(id: string): ScreenPoint | null {
		const a = this.#def?.anchors[id];
		return a ? this.projectPoint(a) : null;
	}

	projectPoint(a: Anchor): ScreenPoint | null {
		if (!this.#root || !this.#def) return null;
		const b = this.#root.getBoundingClientRect();
		return anchorToScreen({ x: b.left, y: b.top, width: b.width, height: b.height }, this.#def.frame, a, this.#camera ?? undefined);
	}

	scale(): number {
		return this.#frameEl ? this.#rect.width * (this.#camera?.zoom ?? 1) : 0;
	}

	onLayout(fn: () => void): () => void {
		this.#listeners.add(fn);
		return () => void this.#listeners.delete(fn);
	}

	setCamera(preset: string, ms = 0): Promise<void> {
		const c = this.#def?.camera?.find((x) => x.id === preset);
		if (!c || !this.#frameEl) return Promise.resolve();
		this.#camera = c;
		const instant = ms <= 0 || this.#quality === 'off';
		this.#frameEl.style.transition = instant ? 'none' : `transform ${ms}ms var(--ease-in-out-sine)`;
		this.#applyCamera();
		this.#notify();
		return instant ? Promise.resolve() : new Promise((r) => setTimeout(r, ms));
	}

	setQuality(q: Quality): void {
		this.#quality = q;
		if (q === 'off') this.#tx = this.#ty = 0;
		this.#syncLoop();
	}

	pause(): void {
		this.#paused = true;
		this.#syncLoop();
	}

	resume(): void {
		this.#paused = false;
		this.#syncLoop();
	}

	#layout() {
		if (!this.#root || !this.#frameEl || !this.#def) return;
		const w = this.#root.clientWidth;
		const h = this.#root.clientHeight;
		this.#rect = frameRect(w, h, this.#def.frame);
		const s = this.#frameEl.style;
		s.left = `${this.#rect.x}px`;
		s.top = `${this.#rect.y}px`;
		s.width = `${this.#rect.width}px`;
		s.height = `${this.#rect.height}px`;
		this.#applyCamera();
		this.#notify();
	}

	#notify() {
		for (const fn of this.#listeners) fn();
	}

	#applyCamera() {
		if (this.#frameEl) this.#frameEl.style.transform = this.#camera ? cameraTransform(this.#rect, this.#camera) : '';
	}

	/** Parallax runs on the shared loop only when it can be seen: mounted, not paused, quality above off, layered. */
	#syncLoop() {
		const want = !!this.#frameEl && !this.#paused && this.#quality !== 'off' && this.#layers.some((l) => l.depth > 0);
		if (want && !this.#unsub) this.#unsub = (this.#loop ??= gameLoop()).subscribe(this.#frame);
		else if (!want && this.#unsub) {
			this.#unsub();
			this.#unsub = null;
		}
	}

	#onPointer = (e: PointerEvent) => {
		if (!this.#root || e.pointerType !== 'mouse') return;
		const r = this.#root.getBoundingClientRect();
		this.#tx = ((e.clientX - r.left) / r.width - 0.5) * 2;
		this.#ty = ((e.clientY - r.top) / r.height - 0.5) * 2;
	};

	#onLeave = () => {
		this.#tx = this.#ty = 0;
	};

	#frame = (dt: number) => {
		const k = 1 - Math.exp(-dt / 120); // ease toward the pointer (~120 ms)
		const dx = this.#tx - this.#cx;
		const dy = this.#ty - this.#cy;
		if (Math.abs(dx) < 1e-3 && Math.abs(dy) < 1e-3) return; // settled: no style writes
		this.#cx += dx * k;
		this.#cy += dy * k;
		const amp = this.#quality === 'lite' ? PARALLAX_PX / 2 : PARALLAX_PX;
		for (const l of this.#layers) l.el.style.transform = `translate(${-this.#cx * l.depth * amp}px, ${-this.#cy * l.depth * amp * 0.5}px)`;
	};
}
