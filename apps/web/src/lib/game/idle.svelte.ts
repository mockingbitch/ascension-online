// Display-only idle clock between server snapshots. GET /character returns cultivation already projected to "now"
// and lastCultivatedAt = the last settlement; from those we animate:
//   shown         — stage cultivation, advancing at the idle rate, bounded by the server-reported rooms
//                   (bình cảnh, idle bucket, daily ceiling) AND by the offline window, never past them;
//   elapsed       — time banked since the last settlement (capped at the offline window).
// The next server response replaces everything; nothing here is sent back or trusted.
import { game } from './store.svelte';

const pad = (n: number) => String(n).padStart(2, '0');
export const hms = (secs: number) => {
	const s = Math.max(0, Math.floor(secs));
	return `${pad(Math.floor(s / 3600))}:${pad(Math.floor((s % 3600) / 60))}:${pad(s % 60)}`;
};

export function useIdleClock() {
	let now = $state(Date.now());
	let fetchedAt = Date.now();

	$effect(() => {
		void game.character; // new snapshot → restart the interpolation window
		fetchedAt = Date.now();
		now = fetchedAt;
		const id = setInterval(() => (now = Date.now()), 1000);
		return () => clearInterval(id);
	});

	const ch = $derived(game.character);
	// offlineCapSeconds from the server already includes a pending Tụ Linh Phù bonus.
	const capSecs = $derived(ch ? ch.offlineCapSeconds : 0);
	const elapsed = $derived(ch ? Math.min(capSecs, (now - Date.parse(ch.lastCultivatedAt)) / 1000) : 0);
	const isFull = $derived(!!ch && elapsed >= capSecs);

	const shown = $derived.by(() => {
		if (!ch) return 0;
		const t = ch.today;
		const room = Math.max(0, Math.min(ch.bottleneckRoomMilli, t.buckets.idle.capMilli - t.buckets.idle.milli, t.ceilingMilli - t.pointsMilli));
		const ceil = ch.capMilli > 0 ? ch.cultivationMilli + room : Math.min(ch.requiredMilli, ch.cultivationMilli + room);
		const elapsedAtFetch = (fetchedAt - Date.parse(ch.lastCultivatedAt)) / 1000;
		const accrueSecs = Math.min((now - fetchedAt) / 1000, Math.max(0, capSecs - elapsedAtFetch));
		return Math.min(ceil, Math.floor(ch.cultivationMilli + (ch.idleRateMilliPerHour * accrueSecs) / 3600));
	});

	return {
		get shown() {
			return shown;
		},
		get elapsedSeconds() {
			return elapsed;
		},
		get elapsedLabel() {
			return hms(elapsed);
		},
		get capSeconds() {
			return capSecs;
		},
		get untilFullLabel() {
			return hms(capSecs - elapsed);
		},
		get isFull() {
			return isFull;
		},
		/** bình cảnh reached: only a breakthrough moves on */
		get atBottleneck() {
			return !!ch && ch.capMilli > 0 && shown >= ch.requiredMilli;
		}
	};
}
