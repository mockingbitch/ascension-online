import { api } from '$lib/api/client';
import { ensureSession } from '$lib/api/session.svelte';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => {
	const tiers = (await api.vipTiers(fetch)).tiers;
	const status = (await ensureSession(fetch)) ? await api.vipStatus(fetch).catch(() => null) : null;
	return { tiers, status };
};
