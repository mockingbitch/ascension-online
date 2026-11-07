import { loadFactionMe, loadFactions } from '$lib/game/faction.svelte';
import type { PageLoad } from './$types';

// Fresh roster/ranking and membership on every visit (shared with the HUD card; the server owns every number).
export const load: PageLoad = async ({ fetch }) => {
	await Promise.all([loadFactions(fetch), loadFactionMe(fetch)]);
};
