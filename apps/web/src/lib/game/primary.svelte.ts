// The scene-contextual primary button of the action bar (spec §3.4): the scene page registers what it is (Động Phủ /
// Thế Giới "Thu nhận", Tử Tiêu Cung "Ngộ đạo") as a getter, so the bar shows live values and calls the page's own
// handler — the bar never duplicates game logic, and the server decides every result.
import type { IconName } from '$lib/components/Icon.svelte';

export interface PrimaryAction {
	id: string;
	label: string;
	icon: IconName;
	/** short status under the label (countdown, "Đã đầy") */
	hint?: string;
	/** something is waiting: the button sweeps (decorative; the hint/label say it in text) */
	ready: boolean;
	disabled: boolean;
	/** its request is in flight: the bar shows "đang gửi" at once (spinner, aria-busy) — the result waits for the server */
	busy?: boolean;
	/** 0..1 ring around the button (display-only clock), omitted = no ring */
	progress?: number;
	tone: 'jade' | 'gold';
	run: () => void;
}

const slot = $state<{ get: (() => PrimaryAction) | null }>({ get: null });

/** Call during a scene page's init: the action is shown while the page is mounted. */
export function usePrimaryAction(get: () => PrimaryAction) {
	$effect(() => {
		slot.get = get;
		return () => {
			if (slot.get === get) slot.get = null;
		};
	});
}

/** The current primary action (reactive), or null. */
export const primaryAction = (): PrimaryAction | null => slot.get?.() ?? null;
