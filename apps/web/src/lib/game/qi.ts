// Pure projection of the displayed tu vi between server snapshots (UI polish U4, A9: the one formula every reader
// uses through lib/game/idle.svelte.ts). Display only — the next server response replaces it.
import type { CharacterView as Character } from './types';

export type Snapshot = Pick<
	Character,
	'cultivationMilli' | 'requiredMilli' | 'capMilli' | 'bottleneckRoomMilli' | 'idleRateMilliPerHour' | 'offlineCapSeconds' | 'lastCultivatedAt' | 'today'
>;

/** Pure: the displayed stage cultivation `now` ms, for a snapshot received at `fetchedAt` ms. */
export function projectQi(ch: Snapshot, fetchedAt: number, now: number): number {
	const t = ch.today;
	const room = Math.max(0, Math.min(ch.bottleneckRoomMilli, t.buckets.idle.capMilli - t.buckets.idle.milli, t.ceilingMilli - t.pointsMilli));
	const ceil = ch.capMilli > 0 ? ch.cultivationMilli + room : Math.min(ch.requiredMilli, ch.cultivationMilli + room);
	const elapsedAtFetch = (fetchedAt - Date.parse(ch.lastCultivatedAt)) / 1000;
	const accrueSecs = Math.max(0, Math.min((now - fetchedAt) / 1000, Math.max(0, ch.offlineCapSeconds - elapsedAtFetch)));
	return Math.min(ceil, Math.floor(ch.cultivationMilli + (ch.idleRateMilliPerHour * accrueSecs) / 3600));
}
