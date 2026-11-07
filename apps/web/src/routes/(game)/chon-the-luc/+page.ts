import { api } from '$lib/api/client';
import type { PageLoad } from './$types';

// Live roster (the game guard only lets characters without a faction in here).
export const load: PageLoad = async ({ fetch, parent }) => {
	await parent();
	return { list: await api.factions(fetch) };
};
