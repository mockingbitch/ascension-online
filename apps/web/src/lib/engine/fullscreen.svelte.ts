// Fullscreen toggle (spec §4.11): the Fullscreen API with the webkit-prefixed fallback (older Safari / iPadOS).
// Where neither exists (Safari on iPhone) `supported` is false and the buttons are hidden — the PWA (Thêm vào màn
// hình chính) is the full-screen route there. Presentation only.

type WebkitDoc = Document & {
	webkitFullscreenEnabled?: boolean;
	webkitFullscreenElement?: Element | null;
	webkitExitFullscreen?: () => Promise<void> | void;
};
type WebkitEl = HTMLElement & { webkitRequestFullscreen?: () => Promise<void> | void };

/** Which API this document offers. Pure over the given objects (unit-tested). */
export function fullscreenApi(doc: Partial<WebkitDoc> | undefined, el: Partial<WebkitEl> | undefined): 'std' | 'webkit' | null {
	if (!doc || !el) return null;
	if (doc.fullscreenEnabled && typeof el.requestFullscreen === 'function') return 'std';
	if (doc.webkitFullscreenEnabled && typeof el.webkitRequestFullscreen === 'function') return 'webkit';
	return null;
}

class Fullscreen {
	supported = $state(false);
	active = $state(false);
	#installed = false;

	/** Detect support and follow fullscreenchange (call once in the browser). */
	install(): () => void {
		if (typeof document === 'undefined' || this.#installed) return () => {};
		this.#installed = true;
		const doc = document as WebkitDoc;
		this.supported = fullscreenApi(doc, document.documentElement) !== null;
		const sync = () => (this.active = !!(doc.fullscreenElement ?? doc.webkitFullscreenElement));
		sync();
		doc.addEventListener('fullscreenchange', sync);
		doc.addEventListener('webkitfullscreenchange', sync);
		return () => {
			this.#installed = false;
			doc.removeEventListener('fullscreenchange', sync);
			doc.removeEventListener('webkitfullscreenchange', sync);
		};
	}

	/** Enter / leave fullscreen (call from a click: the API needs a user gesture). */
	async toggle(): Promise<void> {
		const doc = document as WebkitDoc;
		const el = document.documentElement as WebkitEl;
		const api = fullscreenApi(doc, el);
		try {
			if (doc.fullscreenElement ?? doc.webkitFullscreenElement) {
				if (doc.exitFullscreen) await doc.exitFullscreen();
				else await doc.webkitExitFullscreen?.();
			} else if (api === 'std') await el.requestFullscreen({ navigationUI: 'hide' });
			else if (api === 'webkit') await el.webkitRequestFullscreen?.();
		} catch {
			/* refused (no gesture, iframe policy): stay as is */
		}
	}
}

export const fullscreen = new Fullscreen();
