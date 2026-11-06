import { api } from '$lib/api/client';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => {
	try {
		return { catalogue: await api.realms(fetch), error: null };
	} catch (e) {
		return { catalogue: null, error: (e as Error).message };
	}
};
