import { redirect } from '@sveltejs/kit';
import { guardTarget } from '$lib/game/onboarding';
import type { LayoutLoad } from './$types';

// Onboarding guard for the whole /play area: no character → /create, no faction → /chon-the-luc. It reads only the
// parent data (never the URL), so moving between scenes, screens and windows re-runs nothing.
export const load: LayoutLoad = async ({ parent }) => {
	const { character } = await parent();
	const to = guardTarget('/play', character);
	if (to) redirect(307, to);
	return {};
};
