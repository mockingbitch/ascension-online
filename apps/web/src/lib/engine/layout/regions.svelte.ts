// The /play shell's measured regions (UI polish U2): one instance, written by the shell's single ResizeObserver,
// read by whatever must stay inside the stage (scene-anchored plaques) or below the HUD (toasts). CSS reads the same
// numbers through the custom properties regionVars() publishes on <html>.
import { chatBox, regionVars, stageRect, type RegionSizes } from './regions';

class Regions {
	sizes = $state<RegionSizes>({ vw: 0, vh: 0, hud: 56, bar: 0, rail: 0, chatW: 0, chatH: 0, safeB: 0, wide: true });
	readonly stage = $derived(stageRect(this.sizes));
	readonly chat = $derived(chatBox(this.sizes));

	/** Measure the shell's region elements and publish the variables (called on resize only). */
	measure(els: { hud?: HTMLElement; bar?: HTMLElement; chat?: HTMLElement }, wide: boolean) {
		const h = (el?: HTMLElement) => (el ? el.getBoundingClientRect().height : 0);
		const w = (el?: HTMLElement) => (el ? el.getBoundingClientRect().width : 0);
		// the bar region pads itself by env(safe-area-inset-bottom) on phones
		const safeB = !wide && els.bar ? parseFloat(getComputedStyle(els.bar).paddingBottom) || 0 : 0;
		const next: RegionSizes = {
			vw: innerWidth,
			vh: innerHeight,
			hud: h(els.hud),
			bar: wide ? 0 : Math.max(0, h(els.bar) - safeB),
			rail: wide ? w(els.bar) : 0,
			chatW: w(els.chat),
			chatH: h(els.chat),
			safeB,
			wide
		};
		const cur = this.sizes;
		if ((Object.keys(next) as (keyof RegionSizes)[]).every((k) => Math.round(Number(cur[k])) === Math.round(Number(next[k])))) return;
		this.sizes = next;
		const style = document.documentElement.style;
		for (const [k, v] of Object.entries(regionVars(next))) style.setProperty(k, v);
	}

	clear() {
		const style = document.documentElement.style;
		for (const k of Object.keys(regionVars(this.sizes))) style.removeProperty(k);
	}
}

export const regions = new Regions();
