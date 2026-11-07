import { api } from '$lib/api/client';
import type { PageLoad } from './$types';

// Bí cảnh status (runs left today, loot tables, caps, rules snapshot) — the server decides everything shown here.
export const load: PageLoad = async ({ fetch }) => {
	const status = await api.secretRealms(fetch).catch(() => null);
	return { status };
};
