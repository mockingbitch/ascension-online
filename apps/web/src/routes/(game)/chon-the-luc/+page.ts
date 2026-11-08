import { redirect } from '@sveltejs/kit';
import { api } from '$lib/api/client';
import { guardTarget } from '$lib/game/onboarding';
import type { PageLoad } from './$types';

// Live roster. Guard: only characters without a faction belong here (no character → /create, member → Thế Lực).
export const load: PageLoad = async ({ fetch, parent }) => {
	const { character } = await parent();
	const to = guardTarget('/chon-the-luc', character);
	if (to) redirect(307, to);
	return { list: await api.factions(fetch) };
};
