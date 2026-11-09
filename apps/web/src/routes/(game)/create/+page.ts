import { redirect } from '@sveltejs/kit';
import { api } from '$lib/api/client';
import { guardTarget } from '$lib/game/onboarding';
import type { PageLoad } from './$types';

// Onboarding wizard: live faction roster (public GET /factions). Nothing is written until the last step.
// Guard: a player who already has a character skips the wizard.
export const load: PageLoad = async ({ fetch, parent }) => {
	const { character } = await parent();
	const to = guardTarget('/create', character);
	if (to) redirect(307, to);
	// The look step needs the appearance catalogue (public, served with the cosmetics catalogue); without it the step
	// is skipped and the server gives the path default.
	const [list, cosmetics] = await Promise.all([api.factions(fetch), api.cosmeticCatalog(fetch).catch(() => null)]);
	return { list, looks: cosmetics?.appearances ?? null };
};
