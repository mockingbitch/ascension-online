// REST client for /api/v1. Every mutation sends an Idempotency-Key; the response is the authoritative
// state and must REPLACE local state (never merge client-computed values).
import type {
	BonusStatus,
	BreakthroughPreview,
	BreakthroughResult,
	DailyBoard,
	DailyTask,
	GiftResult,
	Inventory,
	CharacterView,
	CultivationStatus,
	ItemDef,
	LeaderboardPage,
	RealmCatalogue,
	User,
	VipStatus,
	VipTier
} from '$lib/game/types';
import { ensureSession, refreshSession, session } from './session.svelte';

export class ApiError extends Error {
	constructor(
		public status: number,
		public code: string,
		message: string,
		public details?: unknown
	) {
		super(message);
	}
}

type Fetch = typeof fetch;

interface Opts extends RequestInit {
	auth?: boolean;
}

async function request<T>(f: Fetch, path: string, opts: Opts = {}): Promise<T> {
	const { auth = false, ...init } = opts;
	const send = async () => {
		const headers: Record<string, string> = { Accept: 'application/json', ...(init.headers as Record<string, string>) };
		if (auth && session.accessToken) headers.Authorization = `Bearer ${session.accessToken}`;
		return f(`/api/v1${path}`, { ...init, headers });
	};
	if (auth) await ensureSession(f);
	let res = await send();
	if (res.status === 401 && auth && (await refreshSession(f))) res = await send(); // retry once
	if (!res.ok) {
		const body = await res.json().catch(() => ({}));
		throw new ApiError(res.status, body?.error?.code ?? 'unknown', body?.error?.message ?? res.statusText, body?.error?.details);
	}
	if (res.status === 204) return undefined as T;
	return res.json() as Promise<T>;
}

export const newIdempotencyKey = () => crypto.randomUUID().replaceAll('-', '');

const mutate = (key = newIdempotencyKey()) => ({ method: 'POST', auth: true, headers: { 'Idempotency-Key': key } });

export const api = {
	// public catalogue
	realms: (f: Fetch = fetch) => request<RealmCatalogue>(f, '/realms'),
	items: (f: Fetch = fetch) => request<{ items: ItemDef[] }>(f, '/items'),
	vipTiers: (f: Fetch = fetch) => request<{ tiers: VipTier[] }>(f, '/vip/tiers'),
	leaderboard: (offset = 0, limit = 50, f: Fetch = fetch) =>
		request<LeaderboardPage>(f, `/leaderboards/global?offset=${offset}&limit=${limit}`),

	// account
	me: (f: Fetch = fetch) => request<User>(f, '/me', { auth: true }),
	vipStatus: (f: Fetch = fetch) => request<VipStatus>(f, '/vip/status', { auth: true }),
	myRank: (f: Fetch = fetch) =>
		request<{ rank: number; score: number; characterId: string }>(f, '/leaderboards/global/me', { auth: true }),

	// character
	character: (f: Fetch = fetch) => request<CharacterView>(f, '/character', { auth: true }),
	createCharacter: (name: string, f: Fetch = fetch) =>
		request<CharacterView>(f, '/character', {
			method: 'POST',
			auth: true,
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ name })
		}),

	// gameplay
	settle: (f: Fetch = fetch, key?: string) => request<CultivationStatus>(f, '/cultivation/settle', mutate(key)),
	breakthroughPreview: (f: Fetch = fetch) => request<BreakthroughPreview>(f, '/breakthrough/preview', { auth: true }),
	/** acceptLossMilli: the tribulation loss the player saw (guard only; 409 quote_changed if it grew). */
	breakthroughAttempt: (f: Fetch = fetch, key?: string, acceptLossMilli?: number) => {
		const init = mutate(key);
		if (acceptLossMilli === undefined) return request<BreakthroughResult>(f, '/breakthrough/attempt', init);
		return request<BreakthroughResult>(f, '/breakthrough/attempt', {
			...init,
			headers: { ...init.headers, 'Content-Type': 'application/json' },
			body: JSON.stringify({ acceptLossMilli })
		});
	},

	// economy
	inventory: (f: Fetch = fetch) => request<Inventory>(f, '/inventory', { auth: true }),
	useItem: (itemId: string, f: Fetch = fetch, key?: string) =>
		request<BonusStatus>(f, `/items/${encodeURIComponent(itemId)}/use`, mutate(key)),
	claimVipGift: (f: Fetch = fetch) => request<GiftResult>(f, '/vip/daily-gift', { method: 'POST', auth: true }),
	dailyTasks: (f: Fetch = fetch) => request<DailyBoard>(f, '/tasks/daily', { auth: true }),
	claimTask: (taskId: string, f: Fetch = fetch) =>
		request<DailyTask>(f, `/tasks/daily/${encodeURIComponent(taskId)}/claim`, { method: 'POST', auth: true })
};
