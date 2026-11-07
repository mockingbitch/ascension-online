// Presence for hộ pháp (Phase 3.4): while a game screen is visible, tell the server "I am online" every
// pingEverySeconds (from the server; protector.yaml). The server keeps it as a Redis key with a TTL — no ticker and no
// DB write per ping. Hidden tabs stop pinging, so a player who left soon counts as offline.
import { api } from '$lib/api/client';

export function startPresence(): () => void {
	let timer: ReturnType<typeof setTimeout> | undefined;
	let every = 45;
	let stopped = false;
	const tick = async () => {
		if (stopped) return;
		if (document.visibilityState === 'visible') {
			try {
				every = (await api.presencePing()).pingEverySeconds || every;
			} catch {
				// presence is best effort (503 without Redis, offline network…)
			}
		}
		if (!stopped) timer = setTimeout(tick, every * 1000);
	};
	const onvis = () => {
		if (document.visibilityState === 'visible') {
			clearTimeout(timer);
			void tick();
		}
	};
	document.addEventListener('visibilitychange', onvis);
	void tick();
	return () => {
		stopped = true;
		clearTimeout(timer);
		document.removeEventListener('visibilitychange', onvis);
	};
}
