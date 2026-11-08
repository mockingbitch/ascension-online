// 3D SceneRenderer (GĐ7 spike, Tử Tiêu Cung only): plain Three.js behind the same contract as Render2D. Reached only
// through the dynamic import in ../index.ts (renderer=3d), so 2D players never load it.
//
// - Frame parity: the WebGL canvas fills the scene box; the camera's full image is the def's 16:9 frame placed exactly
//   like the 2D frame (cover / focus via frameRect, presets via viewRect → setViewOffset). Anchors become 3D points
//   on the reference camera's rays (mapping.ts), so at rest anchorToScreen equals the 2D result, and with the idle
//   drift the anchors follow the 3D objects (layout listeners are told when a probe moved ≥ 0.5 px, not every frame).
// - One loop: renders on gameLoop() (renderer.setAnimationLoop(null), never its own rAF), only while visible.
//   full = drift + clouds + shadows, DPR ≤ 2; lite = DPR ≤ 1.5, fewer clouds, no shadows; off = static frames rendered
//   only after a layout change (no drift, no ambient motion).
// - Fallbacks: no WebGL2 → mount throws RendererFailure('no-webgl2'); webglcontextlost / fps < 40 for 5 s → onFail.
// - unmount disposes every geometry/material/texture/render target and drops the GL context.
import { ACESFilmicToneMapping, PCFShadowMap, PerspectiveCamera, SRGBColorSpace, type Texture, WebGLRenderer } from 'three';
import { tokenResolver } from '../../fx/colors';
import type { Quality } from '../../fx/particles';
import { gameLoop, type Loop } from '../../loop';
import type { Anchor, CameraPreset, SceneDef } from '../../scene/types';
import { frameRect, type Rect } from '../frame';
import type { Fallback3dReason } from '../select';
import type { RendererOptions, SceneRenderer, ScreenPoint } from '../types';
import { RendererFailure } from './failure';
import { characterAsset, loadEnv, loadGlb, sceneAsset } from './assets';
import { buildHall, type HallBuild } from './hall';
import { HALL, HALL_REF_POSE } from './layout';
import { anchorToWorld, drift, lerpPreset, viewRect, withOffset, worldToBox, type CameraPose, type Vec3 } from './mapping';
import { readPalette } from './palette';
import { FpsWatchdog } from './watchdog';

/** Max device pixel ratio per quality. */
export const DPR_CAP: Record<Quality, number> = { full: 2, lite: 1.5, off: 1.5 };
/** Layout listeners hear about drift once a probe anchor moved this far (px). */
const NOTIFY_PX = 0.5;

const easeInOut = (t: number) => 0.5 - 0.5 * Math.cos(Math.PI * Math.min(1, Math.max(0, t)));

export class Render3D implements SceneRenderer {
	readonly kind = '3d' as const;
	#root: HTMLElement | null = null;
	#canvas: HTMLCanvasElement | null = null;
	#gl: WebGLRenderer | null = null;
	#cam: PerspectiveCamera | null = null;
	#hall: HallBuild | null = null;
	#env: Texture | null = null;
	#def: SceneDef | null = null;
	#quality: Quality = 'full';
	#preset: CameraPreset | null = null;
	#move: { from: CameraPreset; to: CameraPreset; t0: number | null; ms: number; done: () => void } | null = null;
	#pose: CameraPose = HALL_REF_POSE;
	#rect: Rect = { x: 0, y: 0, width: 0, height: 0 };
	#size = { w: 0, h: 0 };
	#ro: ResizeObserver | null = null;
	#loop: Loop | null;
	#unsub: (() => void) | null = null;
	#listeners = new Set<() => void>();
	#failListeners = new Set<(r: Fallback3dReason) => void>();
	#paused = false;
	#dirty = true;
	#failed = false;
	#compiling = false;
	#guard: FpsWatchdog | null = null;
	#t = 0;
	#probes: Vec3[] = [];
	#probeAt: ({ x: number; y: number } | null)[] = [];
	#gen = 0;
	/** frames rendered (diagnostics: tests read it through the canvas dataset) */
	#frames = 0;

	constructor(loop?: Loop) {
		this.#loop = loop ?? null;
	}

	async mount(el: HTMLElement, def: SceneDef, opts: RendererOptions): Promise<void> {
		this.unmount();
		const gen = ++this.#gen;
		this.#root = el;
		this.#def = def;
		this.#quality = opts.quality;
		this.#preset = def.camera?.[0] ?? null;
		this.#failed = false;
		this.#guard = opts.fpsGuard === false ? null : new FpsWatchdog();
		const canvas = document.createElement('canvas');
		canvas.dataset.scene = def.id;
		canvas.dataset.renderer = '3d';
		canvas.style.cssText = 'position:absolute;inset:0;width:100%;height:100%;display:block;pointer-events:none';
		let gl: WebGLRenderer;
		try {
			// probe WebGL2 ourselves (no console error from three when it is missing), then hand three the context
			const attrs: WebGLContextAttributes = { alpha: true, antialias: true, depth: true, stencil: false, powerPreference: 'high-performance' };
			const context = canvas.getContext('webgl2', attrs);
			if (!context) throw new Error('WebGL2 unavailable');
			gl = new WebGLRenderer({ canvas, context, alpha: true, antialias: true });
		} catch (e) {
			this.#root = null;
			this.#def = null;
			throw new RendererFailure('no-webgl2', e instanceof Error ? e.message : String(e));
		}
		gl.setAnimationLoop(null); // the shared gameLoop() drives every frame
		gl.outputColorSpace = SRGBColorSpace;
		gl.toneMapping = ACESFilmicToneMapping;
		gl.toneMappingExposure = 1.05;
		gl.shadowMap.type = PCFShadowMap;
		gl.shadowMap.enabled = opts.quality === 'full';
		gl.setClearColor(0x000000, 0); // transparent: the scene box's token gradient is the sky
		this.#gl = gl;
		this.#canvas = canvas;
		canvas.addEventListener('webglcontextlost', this.#onLost);

		const colors = tokenResolver();
		try {
			this.#hall = buildHall(def, HALL_REF_POSE, readPalette(colors.resolve), opts.quality);
		} catch (e) {
			this.unmount();
			throw new RendererFailure('error', e instanceof Error ? e.message : String(e));
		} finally {
			colors.dispose();
		}
		this.#cam = new PerspectiveCamera(HALL_REF_POSE.fov, def.frame.aspect, 0.5, 120);
		// probes for drift notifications: the dais and the two outermost cushions of the back row
		this.#probes = ['dais', 'hong_quan', 'seat.0', `seat.${Object.keys(def.anchors).filter((k) => k.startsWith('seat.')).length - 1}`]
			.map((k) => def.anchors[k])
			.filter((a): a is Anchor => !!a)
			.map((a) => anchorToWorld(HALL_REF_POSE, a));

		// Shaders compile off the main thread where KHR_parallel_shader_compile exists (compileAsync); nothing renders
		// until then, so entering the hall never blocks on a synchronous compile (seconds on software GL).
		const t0 = performance.now();
		this.#compiling = true;
		el.append(canvas);
		this.#layout();
		this.#ro = new ResizeObserver(() => this.#layout());
		this.#ro.observe(el);
		try {
			await gl.compileAsync(this.#hall.scene, this.#cam);
		} catch {
			/* compile errors surface on the first render; nothing to do here */
		}
		if (gen !== this.#gen) return; // unmounted meanwhile
		this.#compiling = false;
		canvas.dataset.mountMs = String(Math.round(performance.now() - t0));
		this.#render(0);
		this.#syncLoop();
		await this.#loadSlots(def.id, gen);
	}

	unmount(): void {
		this.#gen++;
		this.#unsub?.();
		this.#unsub = null;
		this.#compiling = false;
		this.#ro?.disconnect();
		this.#ro = null;
		this.#move?.done();
		this.#move = null;
		this.#canvas?.removeEventListener('webglcontextlost', this.#onLost);
		this.#hall?.dispose();
		this.#hall = null;
		this.#env?.dispose();
		this.#env = null;
		if (this.#gl) {
			this.#gl.renderLists.dispose();
			this.#gl.dispose();
			this.#gl.forceContextLoss(); // release the context now, not at GC (20× mount/unmount must not pile them up)
		}
		this.#gl = null;
		this.#cam = null;
		this.#canvas?.remove();
		this.#canvas = null;
		this.#root = null;
		this.#def = null;
		this.#probes = [];
		this.#probeAt = [];
	}

	anchorToScreen(id: string): ScreenPoint | null {
		const a = this.#def?.anchors[id];
		return a ? this.projectPoint(a) : null;
	}

	projectPoint(a: Anchor): ScreenPoint | null {
		if (!this.#root || !this.#def) return null;
		const b = this.#root.getBoundingClientRect();
		const p = worldToBox(this.#pose, viewRect(this.#rect, this.#preset), anchorToWorld(HALL_REF_POSE, a));
		return p ? { x: b.left + p.x, y: b.top + p.y } : null;
	}

	scale(): number {
		return this.#canvas ? this.#rect.width * (this.#preset?.zoom ?? 1) : 0;
	}

	onLayout(fn: () => void): () => void {
		this.#listeners.add(fn);
		return () => void this.#listeners.delete(fn);
	}

	onFail(fn: (reason: Fallback3dReason) => void): () => void {
		this.#failListeners.add(fn);
		return () => void this.#failListeners.delete(fn);
	}

	setCamera(preset: string, ms = 0): Promise<void> {
		const c = this.#def?.camera?.find((x) => x.id === preset);
		if (!c || !this.#canvas) return Promise.resolve();
		this.#move?.done();
		this.#move = null;
		const from = this.#preset ?? c;
		if (ms <= 0 || this.#quality === 'off' || this.#paused) {
			this.#preset = c;
			this.#invalidate();
			return Promise.resolve();
		}
		return new Promise((done) => {
			this.#move = { from, to: c, t0: null, ms, done };
			this.#syncLoop();
		});
	}

	setQuality(q: Quality): void {
		if (q === this.#quality) return;
		this.#quality = q;
		if (this.#gl && this.#hall) {
			const shadows = q === 'full';
			if (this.#gl.shadowMap.enabled !== shadows) {
				this.#gl.shadowMap.enabled = shadows;
				this.#hall.scene.traverse((o) => {
					const m = (o as { material?: { needsUpdate: boolean } | { needsUpdate: boolean }[] }).material;
					for (const x of Array.isArray(m) ? m : m ? [m] : []) x.needsUpdate = true;
				});
			}
			this.#hall.setQuality(q);
			this.#layout();
		}
		this.#guard?.reset();
		this.#syncLoop();
	}

	pause(): void {
		this.#paused = true;
		this.#syncLoop();
	}

	resume(): void {
		this.#paused = false;
		this.#guard?.reset();
		this.#syncLoop();
	}

	/** Diagnostics for the spike's measurements (fps overlay / tests): frames rendered and GPU memory counters. */
	stats() {
		const i = this.#gl?.info;
		return {
			frames: this.#frames,
			calls: i?.render.calls ?? 0,
			triangles: i?.render.triangles ?? 0,
			geometries: i?.memory.geometries ?? 0,
			textures: i?.memory.textures ?? 0,
			dpr: this.#gl?.getPixelRatio() ?? 0,
			shadows: this.#gl?.shadowMap.enabled ?? false
		};
	}

	#layout() {
		if (!this.#root || !this.#gl || !this.#def) return;
		const w = this.#root.clientWidth;
		const h = this.#root.clientHeight;
		this.#rect = frameRect(w, h, this.#def.frame);
		const dpr = Math.min(window.devicePixelRatio || 1, DPR_CAP[this.#quality]);
		if (this.#gl.getPixelRatio() !== dpr) this.#gl.setPixelRatio(dpr);
		if (w !== this.#size.w || h !== this.#size.h) {
			this.#size = { w, h };
			this.#gl.setSize(w, h, false);
		}
		this.#invalidate();
	}

	/** Something moved anchors (resize, preset): re-render (once, when static) and tell the listeners. */
	#invalidate() {
		this.#dirty = true;
		this.#notify();
		this.#syncLoop();
	}

	#notify() {
		this.#probeAt = this.#probes.map((p) => worldToBox(this.#pose, viewRect(this.#rect, this.#preset), p));
		for (const fn of this.#listeners) fn();
	}

	/** On the loop when there is something to draw: continuous (full/lite, a camera move) or one dirty frame. */
	#syncLoop() {
		const animate = this.#quality !== 'off' || !!this.#move;
		const want = !!this.#gl && !this.#paused && !this.#failed && !this.#compiling && (animate || this.#dirty);
		if (want && !this.#unsub) this.#unsub = (this.#loop ??= gameLoop()).subscribe(this.#frame);
		else if (!want && this.#unsub) {
			this.#unsub();
			this.#unsub = null;
		}
	}

	#frame = (dt: number, now: number) => {
		if (!this.#gl || !this.#hall || !this.#cam) return;
		const animate = this.#quality !== 'off';
		if (animate) this.#t += dt;
		let moved = false;
		if (this.#move) {
			const m = this.#move;
			m.t0 ??= now;
			const k = easeInOut((now - m.t0) / m.ms);
			this.#preset = lerpPreset(m.from, m.to, k);
			moved = true;
			if (k >= 1) {
				this.#preset = m.to;
				this.#move = null;
				m.done();
			}
		}
		this.#render(now);
		if (moved) this.#notify();
		else if (animate) this.#notifyIfDrifted();
		this.#dirty = false;
		if (animate && this.#guard?.frame(now)) return this.#fail('slow');
		if (!animate) this.#syncLoop(); // static: one frame, then off the loop until the next layout change
	};

	#render(now: number) {
		const gl = this.#gl;
		const cam = this.#cam;
		const hall = this.#hall;
		if (!gl || !cam || !hall || !this.#size.w || !this.#size.h) return;
		const amp = this.#quality === 'full' ? HALL.drift.full : this.#quality === 'lite' ? HALL.drift.lite : 0;
		this.#pose = amp ? withOffset(HALL_REF_POSE, drift(this.#t, amp)) : HALL_REF_POSE;
		const p = this.#pose;
		cam.position.set(p.position.x, p.position.y, p.position.z);
		cam.lookAt(p.target.x, p.target.y, p.target.z);
		const vr = viewRect(this.#rect, this.#preset);
		cam.setViewOffset(vr.width, vr.height, -vr.x, -vr.y, this.#size.w, this.#size.h);
		cam.updateProjectionMatrix();
		if (this.#quality !== 'off') hall.update(this.#t);
		gl.render(hall.scene, cam);
		this.#frames++;
		const d = this.#canvas?.dataset;
		if (d) {
			// diagnostics for the spike's measurements (cheap attribute writes; counters refreshed every 60 frames)
			d.frames = String(this.#frames);
			if (this.#frames % 60 === 1 || this.#dirty) {
				const st = this.stats();
				d.calls = String(st.calls);
				d.triangles = String(st.triangles);
				d.dpr = String(st.dpr);
				d.shadows = String(st.shadows);
			}
		}
	}

	#notifyIfDrifted() {
		const vr = viewRect(this.#rect, this.#preset);
		for (let i = 0; i < this.#probes.length; i++) {
			const now = worldToBox(this.#pose, vr, this.#probes[i]!);
			const was = this.#probeAt[i];
			if (!now || !was || Math.hypot(now.x - was.x, now.y - was.y) >= NOTIFY_PX) return this.#notify();
		}
	}

	#fail(reason: Fallback3dReason) {
		if (this.#failed) return;
		this.#failed = true;
		this.#syncLoop();
		for (const fn of this.#failListeners) fn(reason);
	}

	#onLost = (e: Event) => {
		e.preventDefault();
		this.#fail('context-lost');
	};

	/** Real art when it ships: model.glb replaces the procedural architecture, env.hdr lights it, the sage's .glb
	 * replaces the placeholder figure (placed on the `hong_quan` anchor). Failures keep the procedural hall. */
	async #loadSlots(scene: string, gen: number) {
		const model = sceneAsset(scene, 'model.glb');
		const env = sceneAsset(scene, 'env.hdr');
		const sage = characterAsset('hong-quan');
		if (!model && !env && !sage) return;
		try {
			const [m, s] = await Promise.all([model ? loadGlb(model) : null, sage ? loadGlb(sage) : null]);
			if (gen !== this.#gen || !this.#hall || !this.#gl) return;
			if (m) {
				this.#hall.architecture.clear();
				this.#hall.architecture.add(m);
			}
			if (s) {
				const at = this.#hall.sageAt;
				this.#hall.sage.clear();
				s.position.set(0, 0, 0);
				this.#hall.sage.add(s);
				this.#hall.sage.position.set(at.x, at.y, at.z);
			}
			if (env) this.#env = await loadEnv(env, this.#gl, this.#hall.scene);
			this.#invalidate();
		} catch (e) {
			console.warn('[render3d] asset slot failed, keeping the procedural hall', e);
		}
	}
}
