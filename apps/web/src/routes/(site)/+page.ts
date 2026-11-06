import { redirect } from '@sveltejs/kit';
import { api } from '$lib/api/client';
import { ensureSession } from '$lib/api/session.svelte';
import type { PageLoad } from './$types';

// Signed-in players go straight into the game; the landing page is for visitors.
export const load: PageLoad = async ({ fetch }) => {
	if (await ensureSession(fetch)) redirect(307, '/play');
	try {
		return { catalogue: await api.realms(fetch), error: null };
	} catch (e) {
		return { catalogue: null, error: (e as Error).message };
	}
};
