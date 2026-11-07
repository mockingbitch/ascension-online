import { api } from '$lib/api/client';
import { ensureSession } from '$lib/api/session.svelte';
import type { PageLoad } from './$types';

// DEV ONLY: the simulated gateway's checkout page (payments.yaml provider `sandbox`). Real providers host their own.
export const load: PageLoad = async ({ fetch, params }) => {
	const signedIn = await ensureSession(fetch);
	const res = signedIn ? await api.paymentOrder(params.orderId, fetch).catch(() => null) : null;
	return { orderId: params.orderId, order: res?.order ?? null, signedIn };
};
