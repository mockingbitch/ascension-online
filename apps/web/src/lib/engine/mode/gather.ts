// "Gathering qi" while a mode waits for the server: motes converge on a point (the seal / the cultivator) on the
// pooled full-screen fxLayer. Presentation only; returns the emitter's removal. Nothing when effects are off.
import { untrack } from 'svelte';
import { fxLayer } from '../fx/layer';
import { fxOverlay } from '../fx/overlay.svelte';
import type { ParticlePreset, Quality } from '../fx/particles';
import { mote } from '../fx/presets';
import { every } from '../scene/view.svelte';

export function gatherQi(centre: () => { x: number; y: number } | null, q: Quality, color = 'var(--color-jade-300)', perSecFull = 18): () => void {
	if (q === 'off') return () => {};
	const perSec = q === 'lite' ? Math.max(1, Math.round(perSecFull / 3)) : perSecFull;
	const preset: ParticlePreset = { ...mote, id: `gather_${color}`, lifeMs: [900, 1300], size: [1.6, 3], colors: [color, 'var(--color-spirit-cyan)'] };
	untrack(() => fxOverlay.raised++); // keep the fx canvas above the mode (untracked: callers are effects)
	return fxLayer.addEmitter(
		every(
			() => 1000 / perSec,
			() => {
				const c = centre();
				if (!c) return;
				const a = Math.random() * Math.PI * 2;
				const r = Math.min(innerWidth, innerHeight) * (0.22 + Math.random() * 0.12);
				fxLayer.path(preset, { from: { x: c.x + Math.cos(a) * r, y: c.y + Math.sin(a) * r }, to: c, ms: 1100, jitter: 24 }, 1);
			}
		)
	);
}
