// DOM side of the FxLayer (spec §3.1): floating numbers, reward icons flying to the bag, light rings, the world
// announcement scroll. A bounded list rendered by FxOverlay.svelte; each item removes itself when its animation ends
// (or after its static time when effects are off). Display only — texts are server values formatted by the director.
import type { IconName } from '$lib/components/Icon.svelte';

export interface Pt {
	x: number;
	y: number;
}

export type OverlayItem =
	| { id: number; kind: 'float'; text: string; at: Pt; tone: 'jade' | 'gold' }
	| { id: number; kind: 'fly'; from: Pt; to: Pt; icon: IconName; art?: string; rarity?: string; rare: boolean; delay: number; ms: number; label: string }
	| { id: number; kind: 'ring'; at: Pt; tone: 'jade' | 'gold' };

export interface ScrollItem {
	id: string;
	text: string;
	title: string;
}

/** At most this many overlay items at once (oldest dropped). */
export const OVERLAY_MAX = 16;
/** Queued announcement scrolls (newest kept). */
export const SCROLL_QUEUE_MAX = 3;

type NewItem = OverlayItem extends infer T ? (T extends OverlayItem ? Omit<T, 'id'> : never) : never;

class FxOverlay {
	items = $state<OverlayItem[]>([]);
	scrolls = $state<ScrollItem[]>([]);
	/** bumps whenever something starts: the FxLayer re-raises itself above open modal windows */
	raised = $state(0);
	#seq = 0;

	add(item: NewItem): number {
		const id = ++this.#seq;
		this.items = [...this.items, { ...item, id } as OverlayItem].slice(-OVERLAY_MAX);
		this.raised++;
		return id;
	}

	remove(id: number) {
		this.items = this.items.filter((i) => i.id !== id);
	}

	pushScroll(s: ScrollItem) {
		if (this.scrolls.some((x) => x.id === s.id)) return;
		this.scrolls = [...this.scrolls, s].slice(-SCROLL_QUEUE_MAX);
		this.raised++;
	}

	/** Skip / finish the scroll on screen. */
	nextScroll() {
		this.scrolls = this.scrolls.slice(1);
	}

	clear() {
		this.items = [];
		this.scrolls = [];
	}
}

export const fxOverlay = new FxOverlay();
