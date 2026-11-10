// Effects director (lazy chunk, started by the game shell): listens to fxBus and runs the commands of planFx at real
// screen positions — scene anchors through the renderer, HUD / action-bar targets by data attributes, the last pointer
// press for "where it came from". It never calls the API and never derives a value: texts and counts come from the
// event (server values). All timers are tracked and cleared on stop; particles go through the pooled fxLayer.
import type { IconName } from '$lib/components/Icon.svelte';
import { art } from '$lib/game/art';
import { hall } from '$lib/game/lecture.svelte';
import { game } from '$lib/game/store.svelte';
import { toast } from '$lib/game/toasts.svelte';
import { btMode } from '../mode/breakthrough.svelte';
import { HALL_DEFAULT_SEATS, hallSeats } from '../scene/defs/tu_tieu_cung';
import { sceneStage } from '../scene/stage.svelte';
import { settings } from '../settings.svelte';
import { STAGE_UP_QUIET_MS } from '../audio/sfxMap';
import { fxBus } from './bus';
import type { FxEventType } from './events';
import { hudFx } from './hud.svelte';
import { fxLayer } from './layer';
import { fxOverlay, type Pt } from './overlay.svelte';
import { planFx, type FxCommand, type FxFrom } from './plan';
import { firework, landSpark, qiOrb } from './presets';

const EVENTS: FxEventType[] = [
	'settled',
	'insight',
	'reward',
	'announcement',
	'worksLevelUp',
	'stageUp',
	'breakthroughStart',
	'breakthrough',
	'breakthroughStalled',
	'breakthroughCancelled'
];
/** A pointer press older than this is not "where the reward came from". */
const POINTER_FRESH_MS = 15_000;

const centre = (el: Element): Pt => {
	const r = el.getBoundingClientRect();
	return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
};
const visible = (el: Element) => {
	// a full-screen mode hides the shell with visibility:hidden (boxes kept): those targets do not count
	if (el.checkVisibility && !el.checkVisibility({ visibilityProperty: true })) return false;
	const r = el.getBoundingClientRect();
	return r.width > 0 && r.height > 0 && r.bottom > 0 && r.right > 0 && r.top < innerHeight && r.left < innerWidth;
};
/** First visible element of `selectors`, in order. */
function target(...selectors: string[]): Element | null {
	for (const s of selectors) for (const el of document.querySelectorAll(s)) if (visible(el)) return el;
	return null;
}

export function itemIcon(itemId: string | undefined, currency: string | undefined): { icon: IconName; art?: string } {
	if (currency) return { icon: 'stone', art: art('items/linh_thach')?.src };
	const id = itemId ?? '';
	const cat = game.catalogue?.items.get(id)?.category;
	const icon: IconName = cat === 'pill' ? 'pill' : cat === 'herb' ? 'lotus' : id.endsWith('_phu') ? 'talisman' : 'spark';
	return { icon, art: art(`items/${id}`)?.src };
}

export function startDirector(): () => void {
	let pointer: (Pt & { at: number }) | null = null;
	const timers = new Set<ReturnType<typeof setTimeout>>();
	const later = (ms: number, fn: () => void) => {
		const t = setTimeout(() => {
			timers.delete(t);
			fn();
		}, ms);
		timers.add(t);
	};
	const onDown = (e: PointerEvent) => (pointer = { x: e.clientX, y: e.clientY, at: performance.now() });
	addEventListener('pointerdown', onDown, { capture: true, passive: true });

	const screenMid = (): Pt => ({ x: innerWidth / 2, y: innerHeight / 2 });
	const primary = () => target('[data-primary]');
	function from(kind: FxFrom): Pt {
		const r = sceneStage.renderer;
		if (kind === 'cultivator' && r && sceneStage.shown === 'dong_phu' && !sceneStage.covered) {
			const p = r.anchorToScreen('dantian');
			if (p) return p;
		}
		if (kind === 'seat' && r && sceneStage.shown === 'tu_tieu_cung') {
			const seat = hall.state?.me.seat ?? -1;
			const s = seat >= 0 ? hallSeats(hall.state?.capacity || HALL_DEFAULT_SEATS, hall.seatFit)[seat] : undefined;
			const p = s && r.projectPoint(s);
			if (p) return p;
		}
		if (kind === 'pointer' && pointer && performance.now() - pointer.at < POINTER_FRESH_MS) return pointer;
		const b = primary();
		return b ? centre(b) : pointer ?? screenMid();
	}
	const qiTarget = () => target('[data-fx-target="qi"]', '[data-chip="stones"]');

	function run(c: FxCommand) {
		switch (c.kind) {
			case 'float': {
				const t = qiTarget();
				fxOverlay.add({ kind: 'float', text: c.text, at: t ? centre(t) : screenMid(), tone: c.tone });
				return;
			}
			case 'orbs': {
				const t = qiTarget();
				const to = t ? centre(t) : { x: innerWidth / 2, y: 24 };
				const start = from(c.from);
				const release = hudFx.hold('qi', c.ms + 900);
				const stagger = Math.min(40, (c.ms * 0.45) / Math.max(1, c.count));
				fxOverlay.raised++;
				fxLayer.burst(landSpark, start.x, start.y, Math.ceil(c.count / 2));
				fxLayer.path(qiOrb, { from: start, to, ms: c.ms, stagger, jitter: 26 }, c.count);
				later(c.ms, () => {
					release();
					const at = t && t.isConnected ? centre(t) : to;
					fxLayer.burst(landSpark, at.x, at.y);
					fxOverlay.add({ kind: 'float', text: c.float, at, tone: 'jade' });
				});
				return;
			}
			case 'fly': {
				const toEl = c.to === 'bag' ? target('[data-bar="inventory"]', '[data-bar="more"]') : target('[data-chip="stones"]');
				const to = toEl ? centre(toEl) : { x: innerWidth - 40, y: innerHeight - 40 };
				const release = c.to === 'stones' ? hudFx.hold('stones', c.delay + c.ms + 600) : null;
				const ic = itemIcon(c.itemId, c.currency);
				const name = c.itemId ? (game.catalogue?.itemName(c.itemId) ?? c.itemId) : 'linh thạch';
				// a mode's result card marks where its rewards come from
				const src = target('[data-fx-source]');
				fxOverlay.add({
					kind: 'fly',
					from: src ? centre(src) : from('pointer'),
					to,
					icon: ic.icon,
					art: ic.art,
					rarity: c.rarity,
					rare: c.rare,
					delay: c.delay,
					ms: c.ms,
					label: `${name} ×${c.quantity.toLocaleString('vi-VN')}`
				});
				later(c.delay + c.ms, () => {
					release?.();
					if (c.to === 'bag') hudFx.bagBounce++;
					const at = toEl?.isConnected ? centre(toEl) : to;
					fxLayer.burst(landSpark, at.x, at.y, c.rare ? 18 : 8);
				});
				return;
			}
			case 'ring': {
				const b = primary();
				fxOverlay.add({ kind: 'ring', at: b ? centre(b) : screenMid(), tone: c.tone });
				return;
			}
			case 'scroll':
				fxOverlay.pushScroll({ id: c.id, title: c.title, text: c.text });
				return;
			case 'mode': {
				if (c.step === 'gather') btMode.begin(c.gather, c.tribulation);
				else if (c.step === 'outcome') btMode.resolve(c.plan, c.result);
				else if (c.step === 'stall') btMode.stall(c.message);
				else btMode.close();
				return;
			}
			case 'stageUp': {
				// the toast carries the information at every level; motion only when the plan asks for it
				toast('ok', c.text, 3500);
				const t = qiTarget();
				if (!t) return;
				const r = t.getBoundingClientRect();
				if (c.particles > 0) {
					fxOverlay.raised++;
					fxLayer.burst(landSpark, r.left + r.width / 2, r.top + r.height / 2, c.particles);
				}
				if (c.sweepMs > 0) sweep(r, c.fxTier, c.sweepMs);
				return;
			}
			case 'firework': {
				const p = from('pointer');
				for (let i = 0; i < c.count; i++)
					later(i * 260, () => {
						fxOverlay.raised++;
						fxLayer.burst(firework, p.x + (i - 1) * 60, p.y - 40 - (i % 2) * 50);
					});
				return;
			}
		}
	}

	// a light sweep along the tu vi bar: one temporary fixed box (transform/opacity only), removed when it ends
	function sweep(r: DOMRect, fxTier: number, ms: number) {
		const box = document.createElement('div');
		box.setAttribute('aria-hidden', 'true');
		box.dataset.fxSweep = String(fxTier);
		Object.assign(box.style, {
			position: 'fixed', left: `${r.left}px`, top: `${r.top}px`, width: `${r.width}px`, height: `${r.height}px`,
			overflow: 'hidden', pointerEvents: 'none', zIndex: 'var(--z-toast)', borderRadius: '4px'
		});
		const bar = document.createElement('div');
		const aura = game.catalogue?.realmOf(game.character?.stageId ?? '')?.visual.aura ?? 'jade';
		Object.assign(bar.style, {
			position: 'absolute', inset: '0', width: '40%',
			background: `linear-gradient(90deg, transparent, color-mix(in oklab, var(--color-aura-${aura}) 70%, var(--color-mist-100)), transparent)`
		});
		box.appendChild(bar);
		document.body.appendChild(box);
		const anim = bar.animate([{ transform: 'translateX(-110%)', opacity: 0.9 }, { transform: 'translateX(260%)', opacity: 0 }], { duration: ms, easing: 'ease-out' });
		const done = () => box.remove();
		anim.onfinish = done;
		later(ms + 60, done);
	}

	const rarityOf = (id: string) => game.catalogue?.items.get(id)?.rarity;
	// display lookups for the plans: the effect tier of a stage's realm (realms.yaml visual.fx_tier) and its name
	const ctx = {
		tierOf: (id: string) => game.catalogue?.realmOf(id)?.visual.fxTier,
		stageName: (id: string) => game.catalogue?.stages.get(id)?.name ?? ''
	};
	const wanted = new Set<FxEventType>(EVENTS);
	let lastBreakthrough = -Infinity;
	const offs = [
		fxBus.onAny((e) => {
			if (!wanted.has(e.type)) return;
			if (e.type === 'breakthrough') lastBreakthrough = performance.now();
			// the stageUp a breakthrough also emits is part of that moment (the mode shows it)
			if (e.type === 'stageUp' && performance.now() - lastBreakthrough < STAGE_UP_QUIET_MS) return;
			for (const c of planFx(e, settings.fx, rarityOf, settings.reducedMotion, ctx)) run(c);
		})
	];

	btMode.attached = true;

	return () => {
		btMode.attached = false;
		for (const off of offs) off();
		for (const t of timers) clearTimeout(t);
		timers.clear();
		removeEventListener('pointerdown', onDown, { capture: true });
		fxOverlay.clear();
	};
}
