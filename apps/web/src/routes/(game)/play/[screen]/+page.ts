import { redirect } from '@sveltejs/kit';
import { windowFromPath, windowHref } from '$lib/game/windowStack';
import type { PageLoad } from './$types';

// Former screen routes (/play/nhan-vat, /play/tui-do?tab=…) are windows now, and unknown places are a "sealed"
// window: a deep link builds the default scene and opens the window over it (the shell reads ?win=…).
export const load: PageLoad = ({ params, url }) => {
	const w = windowFromPath(`/play/${params.screen}`, url.search) ?? { kind: 'sealed' as const, screen: params.screen };
	redirect(307, windowHref(w));
};
