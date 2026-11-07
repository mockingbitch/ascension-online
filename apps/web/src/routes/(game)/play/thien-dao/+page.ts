import { loadWorld } from '$lib/game/world.svelte';
import type { PageLoad } from './$types';

// Fresh story clock on every visit (shared with the HUD indicator; errors render a retry in the page).
export const load: PageLoad = async ({ fetch }) => {
	await loadWorld(true, fetch);
};
