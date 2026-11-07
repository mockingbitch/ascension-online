import { redirect } from '@sveltejs/kit';
import { api, ApiError } from '$lib/api/client';
import { ensureSession } from '$lib/api/session.svelte';
import { CatalogueIndex } from '$lib/game/catalogue';
import { guardTarget } from '$lib/game/onboarding';
import type { CharacterView } from '$lib/game/types';
import type { LayoutLoad } from './$types';

// Guard: signed in → new players go through the onboarding wizard (/create), players with a character but no faction
// land on the faction step (/chon-the-luc), returning players skip both (lib/game/onboarding.ts guardTarget).
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
	const to = guardTarget(url.pathname, character);
	if (to) redirect(307, to);
	return { catalogue: new CatalogueIndex(cat, items.items), character };
};
