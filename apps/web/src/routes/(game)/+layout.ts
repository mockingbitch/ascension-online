import { redirect } from '@sveltejs/kit';
import { api } from '$lib/api/client';
import { ensureSession } from '$lib/api/session.svelte';
import { CatalogueIndex } from '$lib/game/catalogue';
import type { CharacterView } from '$lib/game/types';
import type { LayoutLoad } from './$types';

/** Invalidation key of the character snapshot (onboarding steps invalidate it after writing). */
export const _CHARACTER_DEP = 'app:character';

// Session + catalogue + character, loaded ONCE for the whole game area. This load must not depend on the URL:
// SvelteKit re-runs a load whenever a URL part it read changes, and reading url.pathname here used to re-fetch
// realms, items, the character and (via game.init) the side data on every scene/screen change. The onboarding guard
// lives in the child loads ((game)/play/+layout.ts, /create, /chon-the-luc) against fixed paths.
export const load: LayoutLoad = async ({ fetch, url, untrack, depends }) => {
	depends(_CHARACTER_DEP);
	if (!(await ensureSession(fetch))) {
		redirect(307, `/login?return=${encodeURIComponent(untrack(() => url.pathname))}`);
	}
	const [cat, items] = await Promise.all([api.realms(fetch), api.items(fetch)]);
	const character: CharacterView | null = await api.characterOrNull(fetch);
	return { catalogue: new CatalogueIndex(cat, items.items), character };
};
