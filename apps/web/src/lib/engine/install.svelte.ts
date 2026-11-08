// "Thêm vào màn hình chính" (spec §4.11). Chromium fires `beforeinstallprompt` when the PWA is installable: we keep
// the event and show a small dismissible hint on phones that calls prompt(). iOS Safari has no such event: the hint
// opens short instructions (Chia sẻ → Thêm vào MH chính) instead. Hidden once installed (standalone display mode)
// or dismissed (remembered in localStorage, wrapped in try/catch).

export const INSTALL_DISMISS_KEY = 'tiendao.installHint.v1';

interface InstallPromptEvent extends Event {
	prompt(): Promise<void>;
	userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>;
}

export interface InstallEnv {
	standalone: boolean;
	dismissed: boolean;
	/** a phone/tablet (coarse pointer, narrow screen) */
	mobile: boolean;
	hasPrompt: boolean;
	ios: boolean;
}

/** What the hint offers: the native prompt, iOS instructions, or nothing. Pure (unit-tested). */
export function installHintMode(e: InstallEnv): 'prompt' | 'ios' | null {
	if (e.standalone || e.dismissed || !e.mobile) return null;
	if (e.hasPrompt) return 'prompt';
	return e.ios ? 'ios' : null;
}

/** iPhone / iPad (iPadOS reports a Mac with touch). */
export function isIos(ua: string, maxTouchPoints: number): boolean {
	return /iPhone|iPad|iPod/.test(ua) || (/Macintosh/.test(ua) && maxTouchPoints > 1);
}

function readDismissed(): boolean {
	try {
		return localStorage.getItem(INSTALL_DISMISS_KEY) === '1';
	} catch {
		return false;
	}
}

class Install {
	#prompt: InstallPromptEvent | null = null;
	hasPrompt = $state(false);
	dismissed = $state(false);
	standalone = $state(false);
	mobile = $state(false);
	ios = $state(false);
	#installed = false;

	get mode() {
		return installHintMode({ standalone: this.standalone, dismissed: this.dismissed, mobile: this.mobile, hasPrompt: this.hasPrompt, ios: this.ios });
	}

	install(): () => void {
		if (typeof window === 'undefined' || this.#installed) return () => {};
		this.#installed = true;
		this.dismissed = readDismissed();
		this.ios = isIos(navigator.userAgent, navigator.maxTouchPoints ?? 0);
		const standaloneMq = matchMedia('(display-mode: standalone), (display-mode: fullscreen)');
		const mobileMq = matchMedia('(pointer: coarse) and (max-width: 1024px)');
		const sync = () => {
			this.standalone = standaloneMq.matches || (navigator as Navigator & { standalone?: boolean }).standalone === true;
			this.mobile = mobileMq.matches;
		};
		sync();
		const onPrompt = (e: Event) => {
			e.preventDefault(); // our hint replaces the mini-infobar
			this.#prompt = e as InstallPromptEvent;
			this.hasPrompt = true;
		};
		const onInstalled = () => {
			this.#prompt = null;
			this.hasPrompt = false;
			this.standalone = true;
		};
		addEventListener('beforeinstallprompt', onPrompt);
		addEventListener('appinstalled', onInstalled);
		standaloneMq.addEventListener('change', sync);
		mobileMq.addEventListener('change', sync);
		return () => {
			this.#installed = false;
			removeEventListener('beforeinstallprompt', onPrompt);
			removeEventListener('appinstalled', onInstalled);
			standaloneMq.removeEventListener('change', sync);
			mobileMq.removeEventListener('change', sync);
		};
	}

	/** The native install dialog (Chromium). Call from a click. */
	async prompt(): Promise<void> {
		const p = this.#prompt;
		if (!p) return;
		this.#prompt = null;
		this.hasPrompt = false;
		try {
			await p.prompt();
			const c = await p.userChoice;
			if (c.outcome === 'dismissed') this.dismiss();
		} catch {
			/* already used / not allowed */
		}
	}

	dismiss() {
		this.dismissed = true;
		try {
			localStorage.setItem(INSTALL_DISMISS_KEY, '1');
		} catch {
			/* blocked storage: hidden for this session only */
		}
	}
}

export const installHint = new Install();
