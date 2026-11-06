import { error, redirect } from '@sveltejs/kit';
import { api } from '$lib/api/client';
import { ensureSession, session } from '$lib/api/session.svelte';
import type { ReportStatus } from '$lib/game/types';
import type { PageLoad } from './$types';

const PAGE = 50;
const STATUSES: ReportStatus[] = ['open', 'dismissed', 'actioned'];

// Admin only. The server re-checks the role on every admin call; this guard only avoids rendering a dead page.
export const load: PageLoad = async ({ fetch, url }) => {
	if (!(await ensureSession(fetch))) redirect(307, `/login?return=${encodeURIComponent(url.pathname)}`);
	if (session.user?.role !== 'admin') error(403, 'Chỉ ban quản trị được vào trang này');
	const q = url.searchParams.get('status');
	const status: ReportStatus = STATUSES.find((s) => s === q) ?? 'open';
	const offset = Math.max(0, Number(url.searchParams.get('offset')) || 0);
	const page = await api.adminReports(status, offset, PAGE, fetch);
	return { page, status, pageSize: PAGE };
};
