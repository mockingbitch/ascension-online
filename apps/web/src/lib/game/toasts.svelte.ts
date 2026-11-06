// Transient feedback shown at the top of the game screen (replaces inline status lines).
export type ToastTone = 'ok' | 'warn' | 'err';
export interface Toast {
	id: number;
	tone: ToastTone;
	text: string;
}

export const toasts = $state<{ items: Toast[] }>({ items: [] });

let seq = 0;
const MAX = 3;

export function toast(tone: ToastTone, text: string, ms = tone === 'err' ? 6000 : 4000) {
	const id = ++seq;
	toasts.items = [...toasts.items, { id, tone, text }].slice(-MAX);
	setTimeout(() => dismiss(id), ms);
}

export function dismiss(id: number) {
	toasts.items = toasts.items.filter((t) => t.id !== id);
}
