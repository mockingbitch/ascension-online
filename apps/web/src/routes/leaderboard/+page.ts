import { api, ApiError } from '$lib/api/client';
import { CatalogueIndex } from '$lib/game/catalogue';
import type { PageLoad } from './$types';

const PAGE = 50;

export const load: PageLoad = async ({ fetch, url }) => {
	const offset = Math.max(0, Number(url.searchParams.get('offset')) || 0);
	const [cat, items] = await Promise.all([api.realms(fetch), api.items(fetch)]);
	try {
		const page = await api.leaderboard(offset, PAGE, fetch);
		return { page, catalogue: new CatalogueIndex(cat, items.items), pageSize: PAGE, error: null };
	} catch (e) {
		return { page: null, catalogue: new CatalogueIndex(cat, items.items), pageSize: PAGE, error: e instanceof ApiError ? e.message : String(e) };
	}
};
