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
	return { list: await api.factions(fetch) };
};
