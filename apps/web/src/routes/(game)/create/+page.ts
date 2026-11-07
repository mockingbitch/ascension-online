import { api } from '$lib/api/client';
import type { PageLoad } from './$types';

// Onboarding wizard: live faction roster (public GET /factions). Nothing is written until the last step.
export const load: PageLoad = async ({ fetch, parent }) => {
	await parent();
	return { list: await api.factions(fetch) };
};
