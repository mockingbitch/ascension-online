// REST client for /api/v1. Every mutation sends an Idempotency-Key; the response is the authoritative
// state and must REPLACE local state (never merge client-computed values).
import type {
	EquippedArtifact,
	Look,
	TutorialMarkResult,
	TutorialScript,
	TutorialState,
	WorldAnnouncement,
	ProtectorView,
	SecretRealmClaim,
	SecretRealmStart,
	SecretRealmSubmit,
	SecretRealmView,
	WorldView,
	BonusStatus,
	BuyResult,
	FortuneView,
	IncenseResult,
	ShopView,
	SpeedItemResult,
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
	VipTiersResponse,
	AdminReportPage,
	ChatHistory,
	ChatMessage,
	Report,
	ReportReason,
	ReportResolution,
	ReportStatus,
	InsightResult,
	LectureMe,
	LectureScripts,
	LectureState,
	NoteEntry,
	FactionChangeResult,
	FactionList,
	FactionMembersPage,
	FactionMembership,
	PathId,
	TrialClaim,
	TrialRunStart,
	TrialStatus,
	TrialSubmitResult,
	Appearance,
	CosmeticBuyResult,
	CosmeticCatalog,
	CosmeticEquipResult,
	CosmeticsView,
	FactionDonateResult,
	FactionWorksView,
	AdminPaymentOrder,
	AdminPaymentRefundResult,
	PaymentEvent,
	PaymentOrder,
	PaymentOrderResult,
	PaymentPackagesView,
	SandboxCompleteResult
} from '$lib/game/types';
import type { DaoLuView, SongTuClaimResult, SongTuView } from '$lib/game/types';
import type { TradeLineInput, TradeOffer, TradePeer, TradeView } from '$lib/game/types';
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

function factionChange(action: 'join' | 'switch' | 'split-choice', body: { factionId: string; path?: PathId }, key: string, f: Fetch) {
	const init = mutate(key);
	return request<FactionChangeResult>(f, `/factions/${action}`, {
		...init,
		headers: { ...init.headers, 'Content-Type': 'application/json' },
		body: JSON.stringify(body)
	});
}

export const api = {
	// public catalogue
	realms: (f: Fetch = fetch) => request<RealmCatalogue>(f, '/realms'),
	items: (f: Fetch = fetch) => request<{ items: ItemDef[] }>(f, '/items'),
	vipTiers: (f: Fetch = fetch) => request<VipTiersResponse>(f, '/vip/tiers'),
	world: (f: Fetch = fetch) => request<WorldView>(f, '/world'),
	leaderboard: (offset = 0, limit = 50, f: Fetch = fetch) =>
		request<LeaderboardPage>(f, `/leaderboards/global?offset=${offset}&limit=${limit}`),

	// account
	me: (f: Fetch = fetch) => request<User>(f, '/me', { auth: true }),
	vipStatus: (f: Fetch = fetch) => request<VipStatus>(f, '/vip/status', { auth: true }),
	myRank: (f: Fetch = fetch) =>
		request<{ rank: number; score: number; characterId: string }>(f, '/leaderboards/global/me', { auth: true }),

	// character
	character: (f: Fetch = fetch) => request<CharacterView>(f, '/character', { auth: true }),
	/** null when the account has no character yet (204 instead of a 404 — no console error before onboarding) */
	characterOrNull: async (f: Fetch = fetch) =>
		(await request<CharacterView | undefined>(f, '/character?optional=1', { auth: true })) ?? null,
	createCharacter: (name: string, f: Fetch = fetch) =>
		request<CharacterView>(f, '/character', {
			method: 'POST',
			auth: true,
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ name })
		}),
	/** Onboarding intent (data-model §9.6): create the character and join the faction in one transaction. Reuse `key`
	 *  when retrying after a network error (a replay answers 200 with `replayed: true`). */
	onboard: (name: string, factionId: string, key: string, path?: PathId, appearance?: Look, f: Fetch = fetch) => {
		const init = mutate(key);
		return request<CharacterView>(f, '/character', {
			...init,
			headers: { ...init.headers, 'Content-Type': 'application/json' },
			body: JSON.stringify({ name, factionId, path, appearance })
		});
	},

	// gameplay
	settle: (f: Fetch = fetch, key?: string) => request<CultivationStatus>(f, '/cultivation/settle', mutate(key)),
	breakthroughPreview: (f: Fetch = fetch) => request<BreakthroughPreview>(f, '/breakthrough/preview', { auth: true }),
	/** acceptLossMilli: the tribulation loss the player saw (guard only; 409 quote_changed if it grew). */
	/** Guards (compare-only): acceptLossMilli = the loss bound the player saw; acceptSuccessBp = the odds the player saw
	 *  (409 quote_changed + fresh preview if the loss grew or the odds dropped). */
	breakthroughAttempt: (f: Fetch = fetch, key?: string, acceptLossMilli?: number, acceptSuccessBp?: number) => {
		const init = mutate(key);
		if (acceptLossMilli === undefined && acceptSuccessBp === undefined) return request<BreakthroughResult>(f, '/breakthrough/attempt', init);
		return request<BreakthroughResult>(f, '/breakthrough/attempt', {
			...init,
			headers: { ...init.headers, 'Content-Type': 'application/json' },
			body: JSON.stringify({ acceptLossMilli, acceptSuccessBp })
		});
	},

	// economy
	inventory: (f: Fetch = fetch) => request<Inventory>(f, '/inventory', { auth: true }),
	/** Offline-bonus items answer BonusStatus, speed items (instantPointsMilli) SpeedItemResult. */
	useItem: <T extends BonusStatus | SpeedItemResult = BonusStatus>(itemId: string, f: Fetch = fetch, key?: string) =>
		request<T>(f, `/items/${encodeURIComponent(itemId)}/use`, mutate(key)),
	shop: (f: Fetch = fetch) => request<ShopView>(f, '/shop', { auth: true }),
	/** Reuse `key` when retrying the same purchase after a network error (idempotent). */
	shopBuy: (listingId: string, lots: number, key: string, f: Fetch = fetch) => {
		const init = mutate(key);
		return request<BuyResult>(f, '/shop/buy', {
			...init,
			headers: { ...init.headers, 'Content-Type': 'application/json' },
			body: JSON.stringify({ listingId, lots })
		});
	},
	trials: (f: Fetch = fetch) => request<TrialStatus>(f, '/trials', { auth: true }),
	/** Issues a seeded run, or resumes the open one (same seed) — 201 new, 200 resumed. */
	trialStart: (f: Fetch = fetch, kind: string, key = newIdempotencyKey()) =>
		request<TrialRunStart>(f, `/trials/${encodeURIComponent(kind)}/start`, { method: 'POST', auth: true, headers: { 'Idempotency-Key': key } }),
	/** Single use: the server replays the input log and returns the verdict. Retry with the SAME key. */
	trialSubmit: (f: Fetch = fetch, runId: string, claim: TrialClaim, key = newIdempotencyKey()) =>
		request<TrialSubmitResult>(f, `/trials/${encodeURIComponent(runId)}/submit`, {
			method: 'POST',
			auth: true,
			headers: { 'Idempotency-Key': key, 'Content-Type': 'application/json' },
			body: JSON.stringify(claim)
		}),
	// bí cảnh (Phase 3.4): server-issued seeded run, replayed movement log, capped loot
	secretRealms: (f: Fetch = fetch) => request<SecretRealmView>(f, '/secret-realms', { auth: true }),
	/** Issues a seeded run (counts toward runs/day), or resumes the open one (same seed) — 201 new, 200 resumed. */
	secretRealmStart: (f: Fetch = fetch, realmId: string, key = newIdempotencyKey()) =>
		request<SecretRealmStart>(f, `/secret-realms/${encodeURIComponent(realmId)}/start`, { method: 'POST', auth: true, headers: { 'Idempotency-Key': key } }),
	/** Single use: the server replays the moves, judges and grants the loot. Retry with the SAME key. */
	secretRealmSubmit: (f: Fetch = fetch, runId: string, claim: SecretRealmClaim, key = newIdempotencyKey()) =>
		request<SecretRealmSubmit>(f, `/secret-realms/runs/${encodeURIComponent(runId)}/submit`, {
			method: 'POST',
			auth: true,
			headers: { 'Idempotency-Key': key, 'Content-Type': 'application/json' },
			body: JSON.stringify(claim)
		}),
	// pháp bảo slot
	artifacts: (f: Fetch = fetch) => request<EquippedArtifact>(f, '/artifacts', { auth: true }),
	/** itemId '' = unequip. Sets the slot (naturally idempotent). */
	equipArtifact: (itemId: string, f: Fetch = fetch, key = newIdempotencyKey()) =>
		request<EquippedArtifact>(f, '/artifacts/equip', {
			method: 'POST',
			auth: true,
			headers: { 'Idempotency-Key': key, 'Content-Type': 'application/json' },
			body: JSON.stringify({ itemId })
		}),
	// hộ pháp
	protector: (f: Fetch = fetch) => request<ProtectorView>(f, '/protector', { auth: true }),
	protectorInvite: (f: Fetch = fetch, key = newIdempotencyKey()) =>
		request<ProtectorView>(f, '/protector/invite', { method: 'POST', auth: true, headers: { 'Idempotency-Key': key } }),
	protectorAccept: (inviteId: string, f: Fetch = fetch, key = newIdempotencyKey()) =>
		request<ProtectorView>(f, `/protector/invites/${encodeURIComponent(inviteId)}/accept`, { method: 'POST', auth: true, headers: { 'Idempotency-Key': key } }),
	/** "I am online" for hộ pháp (Redis TTL). Fire and forget. */
	presencePing: (f: Fetch = fetch) => request<{ online: boolean; pingEverySeconds: number }>(f, '/presence/ping', { method: 'POST', auth: true }),
	fortune: (f: Fetch = fetch) => request<FortuneView>(f, '/fortune', { auth: true }),
	/** Once per game day; a repeat returns today's draw (replayed). */
	fortuneDraw: (f: Fetch = fetch) => request<FortuneView>(f, '/fortune/draw', { method: 'POST', auth: true }),
	/** Once per game day; a repeat returns today's offering (replayed). */
	incense: (f: Fetch = fetch) => request<IncenseResult>(f, '/tasks/daily/incense', { method: 'POST', auth: true }),
	claimVipGift: (f: Fetch = fetch) => request<GiftResult>(f, '/vip/daily-gift', { method: 'POST', auth: true }),
	dailyTasks: (f: Fetch = fetch) => request<DailyBoard>(f, '/tasks/daily', { auth: true }),
	claimTask: (taskId: string, f: Fetch = fetch) =>
		request<DailyTask>(f, `/tasks/daily/${encodeURIComponent(taskId)}/claim`, { method: 'POST', auth: true }),

	// chat
	chatHistory: (channel: string, before = 0, f: Fetch = fetch) =>
		request<ChatHistory>(f, `/chat/${encodeURIComponent(channel)}/messages${before ? `?before=${before}` : ''}`, { auth: true }),
	/** Reuse `key` when retrying the same message after a network error (idempotent). */
	chatSend: (channel: string, text: string, key: string, f: Fetch = fetch) => {
		const init = mutate(key);
		return request<ChatMessage>(f, `/chat/${encodeURIComponent(channel)}/messages`, {
			...init,
			headers: { ...init.headers, 'Content-Type': 'application/json' },
			body: JSON.stringify({ text })
		});
	},

	// Tử Tiêu Cung
	lectureScripts: (f: Fetch = fetch) => request<LectureScripts>(f, '/lecture/scripts'),
	lectureState: (f: Fetch = fetch) => request<LectureState>(f, '/lecture/state', { auth: true }),
	lectureEnter: (f: Fetch = fetch) => request<LectureState>(f, '/lecture/enter', { method: 'POST', auth: true }),
	lectureLeave: (f: Fetch = fetch, keepalive = false) =>
		request<{ inHall: boolean }>(f, '/lecture/leave', { method: 'POST', auth: true, keepalive }),
	lectureHeartbeat: (f: Fetch = fetch) => request<LectureMe>(f, '/lecture/heartbeat', { method: 'POST', auth: true }),
	/** Reuse `key` when retrying after a network error; the ngộ đạo slot itself is the server-side guard. */
	lectureInsight: (key: string, f: Fetch = fetch) => request<InsightResult>(f, '/lecture/insight', mutate(key)),
	lectureNotebook: (f: Fetch = fetch) => request<{ entries: NoteEntry[] }>(f, '/lecture/notebook', { auth: true }),

	// factions
	factions: (f: Fetch = fetch) => request<FactionList>(f, '/factions'),
	factionMe: (f: Fetch = fetch) => request<FactionMembership>(f, '/factions/me', { auth: true }),
	factionMembers: (factionId: string, cursor = '', f: Fetch = fetch) =>
		request<FactionMembersPage>(
			f,
			`/factions/${encodeURIComponent(factionId)}/members${cursor ? `?cursor=${encodeURIComponent(cursor)}` : ''}`,
			{ auth: true }
		),
	/** Reuse `key` when retrying the same join/switch/choice after a network error (idempotent). */
	joinFaction: (factionId: string, key: string, path?: PathId, f: Fetch = fetch) => factionChange('join', { factionId, path }, key, f),
	switchFaction: (factionId: string, key: string, path?: PathId, f: Fetch = fetch) =>
		factionChange('switch', { factionId, path }, key, f),
	chooseSplit: (factionId: string, key: string, f: Fetch = fetch) => factionChange('split-choice', { factionId }, key, f),
	factionWorks: (f: Fetch = fetch) => request<FactionWorksView>(f, '/factions/works', { auth: true }),
	/** Visual faction works only. Reuse `key` when retrying the same donation after a network error (idempotent). */
	factionDonate: (amount: number, key: string, f: Fetch = fetch) => {
		const init = mutate(key);
		return request<FactionDonateResult>(f, '/factions/donate', {
			...init,
			headers: { ...init.headers, 'Content-Type': 'application/json' },
			body: JSON.stringify({ amount })
		});
	},

	// cosmetics (no gameplay effect)
	cosmeticCatalog: (f: Fetch = fetch) => request<CosmeticCatalog>(f, '/cosmetics/catalog'),

	/** the world's recent news (public; GĐ-D) */
	recentAnnouncements: (f: Fetch = fetch) => request<{ items: WorldAnnouncement[] }>(f, '/announcements/recent'),

	// tutorial (Vân Khê; display only — a step never grants anything)
	tutorialScript: (f: Fetch = fetch) => request<TutorialScript>(f, '/tutorial/script'),
	tutorial: (f: Fetch = fetch) => request<TutorialState>(f, '/tutorial', { auth: true }),
	/** Mark a stored step seen (`done`) or dismissed (`skip`); reuse `key` on a retry. */
	tutorialMark: (stepId: string, how: 'done' | 'skip', key: string, f: Fetch = fetch) =>
		request<TutorialMarkResult>(f, `/tutorial/steps/${encodeURIComponent(stepId)}/${how}`, mutate(key)),
	tutorialSkipAll: (key: string, f: Fetch = fetch) => request<TutorialState>(f, '/tutorial/skip-all', mutate(key)),
	tutorialReset: (key: string, f: Fetch = fetch) => request<TutorialState>(f, '/tutorial/reset', mutate(key)),
	cosmetics: (f: Fetch = fetch) => request<CosmeticsView>(f, '/cosmetics', { auth: true }),
	/** Reuse `key` when retrying the same purchase after a network error (idempotent). */
	cosmeticBuy: (cosmeticId: string, key: string, f: Fetch = fetch) => {
		const init = mutate(key);
		return request<CosmeticBuyResult>(f, '/cosmetics/buy', {
			...init,
			headers: { ...init.headers, 'Content-Type': 'application/json' },
			body: JSON.stringify({ cosmeticId })
		});
	},
	/** Sets the WHOLE loadout (empty = unequip). */
	cosmeticEquip: (look: Appearance, key: string, f: Fetch = fetch) => {
		const init = mutate(key);
		return request<CosmeticEquipResult>(f, '/cosmetics/equip', {
			...init,
			headers: { ...init.headers, 'Content-Type': 'application/json' },
			body: JSON.stringify({
				titleId: look.titleId ?? '',
				avatarFrameId: look.avatarFrameId ?? '',
				chatAuraId: look.chatAuraId ?? '',
				caveDecorationIds: look.caveDecorationIds
			})
		});
	},

	// moderation
	/** Reuse the same key when retrying the same report (idempotent). */
	report: (targetCharacterId: string, reason: ReportReason, note: string, key: string, chatMessageId = 0, f: Fetch = fetch) => {
		const init = mutate(key);
		return request<Report>(f, '/reports', {
			...init,
			headers: { ...init.headers, 'Content-Type': 'application/json' },
			body: JSON.stringify({ targetCharacterId, reason, note, chatMessageId })
		});
	},
	adminHideChat: (messageId: number, reason: string, f: Fetch = fetch) =>
		request<{ id: number; hidden: boolean }>(f, `/admin/chat/messages/${messageId}/hide`, {
			method: 'POST',
			auth: true,
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ reason })
		}),
	adminMuteChat: (userId: string, hours: number, reason: string, f: Fetch = fetch) =>
		request<{ userId: string; mutedUntil: string }>(f, '/admin/chat/mutes', {
			method: 'POST',
			auth: true,
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ userId, hours, reason })
		}),
	adminReports: (status: ReportStatus = 'open', offset = 0, limit = 50, f: Fetch = fetch) =>
		request<AdminReportPage>(f, `/admin/reports?status=${status}&offset=${offset}&limit=${limit}`, { auth: true }),
	adminResolveReport: (id: string, outcome: 'dismissed' | 'actioned', note: string, renameTo = '', f: Fetch = fetch) =>
		request<ReportResolution>(f, `/admin/reports/${encodeURIComponent(id)}/resolve`, {
			method: 'POST',
			auth: true,
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ outcome, note, renameTo })
		}),

	// nạp Tiên Duyên (Phase 4). Intent only: the server prices the package; only a verified webhook credits.
	paymentPackages: (f: Fetch = fetch) => request<PaymentPackagesView>(f, '/payments/packages'),
	/** Reuse `key` when retrying the same order after a network error (same key → same order). */
	createPaymentOrder: (packageId: string, key: string, f: Fetch = fetch) => {
		const init = mutate(key);
		return request<PaymentOrderResult>(f, '/payments/orders', {
			...init,
			headers: { ...init.headers, 'Content-Type': 'application/json' },
			body: JSON.stringify({ packageId })
		});
	},
	paymentOrder: (id: string, f: Fetch = fetch) => request<PaymentOrderResult>(f, `/payments/orders/${encodeURIComponent(id)}`, { auth: true }),
	paymentOrders: (f: Fetch = fetch) => request<{ orders: PaymentOrder[] }>(f, '/payments/orders', { auth: true }),
	/** DEV ONLY (404 elsewhere): the sandbox gateway's "pay now / fail" button. deliver=false simulates a lost webhook. */
	sandboxComplete: (orderId: string, outcome: 'paid' | 'failed', deliver = true, f: Fetch = fetch) =>
		request<SandboxCompleteResult>(f, `/dev/payments/sandbox/${encodeURIComponent(orderId)}`, {
			method: 'POST',
			auth: true,
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ outcome, deliver })
		}),
	adminPaymentOrders: (params: { status?: string; userId?: string; q?: string } = {}, f: Fetch = fetch) => {
		const qs = new URLSearchParams(Object.entries(params).filter((e): e is [string, string] => !!e[1]));
		return request<{ orders: AdminPaymentOrder[] }>(f, `/admin/payments/orders?${qs}`, { auth: true });
	},
	adminPaymentOrder: (id: string, f: Fetch = fetch) =>
		request<{ order: AdminPaymentOrder; events: PaymentEvent[] }>(f, `/admin/payments/orders/${encodeURIComponent(id)}`, { auth: true }),
	adminPaymentAlerts: (f: Fetch = fetch) => request<{ events: PaymentEvent[] }>(f, '/admin/payments/alerts', { auth: true }),
	adminPaymentRefund: (id: string, reason: string, key: string, f: Fetch = fetch) => {
		const init = mutate(key);
		return request<AdminPaymentRefundResult>(f, `/admin/payments/orders/${encodeURIComponent(id)}/refund`, {
			...init,
			headers: { ...init.headers, 'Content-Type': 'application/json' },
			body: JSON.stringify({ reason })
		});
	},
	// ───── Đạo lữ song tu (Phase 5.1) ─────
	daoLu: (f: Fetch = fetch) => request<DaoLuView>(f, '/dao-lu', { auth: true }),
	daoLuPropose: (targetCharacterId: string, key = newIdempotencyKey(), f: Fetch = fetch) => {
		const init = mutate(key);
		return request<DaoLuView>(f, '/dao-lu/proposals', {
			...init,
			headers: { ...init.headers, 'Content-Type': 'application/json' },
			body: JSON.stringify({ targetCharacterId, acknowledgePermanent: true })
		});
	},
	daoLuAccept: (proposalId: string, key = newIdempotencyKey(), f: Fetch = fetch) => {
		const init = mutate(key);
		return request<DaoLuView>(f, `/dao-lu/proposals/${encodeURIComponent(proposalId)}/accept`, {
			...init,
			headers: { ...init.headers, 'Content-Type': 'application/json' },
			body: JSON.stringify({ acknowledgePermanent: true })
		});
	},
	daoLuDecline: (proposalId: string, key = newIdempotencyKey(), f: Fetch = fetch) =>
		request<DaoLuView>(f, `/dao-lu/proposals/${encodeURIComponent(proposalId)}/decline`, mutate(key)),
	daoLuCancel: (proposalId: string, key = newIdempotencyKey(), f: Fetch = fetch) =>
		request<DaoLuView>(f, `/dao-lu/proposals/${encodeURIComponent(proposalId)}/cancel`, mutate(key)),
	songTu: (f: Fetch = fetch) => request<SongTuView>(f, '/dao-lu/song-tu', { auth: true }),
	songTuStart: (f: Fetch = fetch) => request<SongTuView>(f, '/dao-lu/song-tu/start', { method: 'POST', auth: true }),
	songTuHeartbeat: (f: Fetch = fetch) => request<SongTuView>(f, '/dao-lu/song-tu/heartbeat', { method: 'POST', auth: true }),
	songTuClaim: (key = newIdempotencyKey(), f: Fetch = fetch) => request<SongTuClaimResult>(f, '/dao-lu/song-tu/claim', mutate(key)),
	// ───── Trao đổi vật phẩm (Phase 5.2) ─────
	trades: (f: Fetch = fetch) => request<TradeView>(f, '/trades', { auth: true }),
	tradeHistory: (f: Fetch = fetch) => request<{ offers: TradeOffer[] }>(f, '/trades/history', { auth: true }),
	tradePeer: (characterId: string, f: Fetch = fetch) => request<TradePeer>(f, `/trades/peer/${encodeURIComponent(characterId)}`, { auth: true }),
	/** take empty = gift. Reuse `key` when retrying the same offer after a network error (idempotent). */
	tradeCreate: (receiverCharacterId: string, give: TradeLineInput[], take: TradeLineInput[], key = newIdempotencyKey(), f: Fetch = fetch) => {
		const init = mutate(key);
		return request<TradeView>(f, '/trades', {
			...init,
			headers: { ...init.headers, 'Content-Type': 'application/json' },
			body: JSON.stringify({ receiverCharacterId, give, take })
		});
	},
	/** The caller confirmed "không hoàn tác" on the confirm screen. */
	tradeAccept: (offerId: string, key = newIdempotencyKey(), f: Fetch = fetch) => {
		const init = mutate(key);
		return request<TradeView>(f, `/trades/${encodeURIComponent(offerId)}/accept`, {
			...init,
			headers: { ...init.headers, 'Content-Type': 'application/json' },
			body: JSON.stringify({ acknowledgeIrreversible: true })
		});
	},
	tradeDecline: (offerId: string, key = newIdempotencyKey(), f: Fetch = fetch) =>
		request<TradeView>(f, `/trades/${encodeURIComponent(offerId)}/decline`, mutate(key)),
	tradeCancel: (offerId: string, key = newIdempotencyKey(), f: Fetch = fetch) =>
		request<TradeView>(f, `/trades/${encodeURIComponent(offerId)}/cancel`, mutate(key)),
	/** Report the other party of a trade (reason trade_abuse). Reuse `key` when retrying (idempotent). */
	reportTrade: (tradeOfferId: string, note: string, key: string, f: Fetch = fetch) => {
		const init = mutate(key);
		return request<Report>(f, '/reports', {
			...init,
			headers: { ...init.headers, 'Content-Type': 'application/json' },
			body: JSON.stringify({ reason: 'trade_abuse', tradeOfferId, note })
		});
	}
};
