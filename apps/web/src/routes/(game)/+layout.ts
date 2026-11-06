import { redirect } from '@sveltejs/kit';
import { api, ApiError } from '$lib/api/client';
import { ensureSession } from '$lib/api/session.svelte';
import { CatalogueIndex } from '$lib/game/catalogue';
import type { CharacterView } from '$lib/game/types';
import type { LayoutLoad } from './$types';

// Guard: signed in → has character (except on /create).
export const load: LayoutLoad = async ({ fetch, url }) => {
	if (!(await ensureSession(fetch))) {
		redirect(307, `/login?return=${encodeURIComponent(url.pathname)}`);
	}
	const [cat, items] = await Promise.all([api.realms(fetch), api.items(fetch)]);
	let character: CharacterView | null = null;
	try {
		character = await api.character(fetch);
	} catch (e) {
		if (!(e instanceof ApiError && e.status === 404)) throw e;
	}
	if (!character && url.pathname !== '/create') redirect(307, '/create');
	if (character && url.pathname === '/create') redirect(307, '/cultivate');
	return { catalogue: new CatalogueIndex(cat, items.items), character };
};
