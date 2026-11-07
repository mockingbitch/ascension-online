import { api } from '$lib/api/client';
import type { PageLoad } from './$types';

// Trial status (state, cooldown, valid pass, rules snapshot) — the server decides everything shown here.
export const load: PageLoad = async ({ fetch }) => {
	const status = await api.trials(fetch).catch(() => null);
	return { status };
};
