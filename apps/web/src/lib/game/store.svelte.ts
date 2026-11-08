// Shared game state for the one-page shell. Every value comes from the server and is REPLACED by each response
// (never merged with client-computed numbers). Actions keep idempotency keys across network errors so a retry
// replays the same intent.
import { api, ApiError, newIdempotencyKey } from '$lib/api/client';
import { fxBus } from '$lib/engine/fx/bus';
import { RESPONSE_TIMEOUT_MS } from '$lib/engine/mode/machine';
import { btMode } from '$lib/engine/mode/breakthrough.svelte';
import type { CatalogueIndex } from './catalogue';
import { formatBp, formatPoints } from './format';
import { InFlight, TimeoutError } from './inflight';
import { toast } from './toasts.svelte';
import type {
	BreakthroughPreview,
	BreakthroughResult,
	CharacterView,
	DailyBoard,
	Inventory,
	LeaderboardEntry,
	SpeedItemResult,
	VipStatus
} from './types';

const PHU = 'tu_linh_phu';
// 4xx = the server answered and wrote nothing; 5xx/network = unknown outcome → keep the key for the retry.
const definitive = (e: unknown) => e instanceof ApiError && e.status < 500;
const errText = (e: unknown) => (e instanceof ApiError ? e.message : String(e));

/** FxBus: tell effects/audio what the server just reported (presentation only; payloads are server values). */
function emitStages(from: string | undefined, advanced: string[]) {
	const to = advanced.at(-1);
	if (from && to && to !== from) fxBus.emit('stageUp', { from, to });
}

class GameStore {
	catalogue = $state<CatalogueIndex | null>(null);
	character = $state<CharacterView | null>(null);
	preview = $state<BreakthroughPreview | null>(null);
	board = $state<DailyBoard | null>(null);
	inventory = $state<Inventory | null>(null);
	vip = $state<VipStatus | null>(null);
	top = $state<LeaderboardEntry[]>([]);
	myRank = $state<number | null>(null);
	busy = $state(false);
	backlash = $state(0); // bump → scene plays the failed-breakthrough shake
	/** the side data (preview, board, inventory, Tiên Duyên) arrived at least once — the boot screen waits for it */
	ready = $state(false);

	#settleKey: string | null = null;
	#attemptKey: string | null = null;
	#attempt = new InFlight<BreakthroughResult>();
	#attemptsShown = new WeakSet<BreakthroughResult>();
	#speedKeys = new Map<string, string>();
	#initFrom: CharacterView | null = null;

	get phuCount() {
		return this.inventory?.items.find((i) => i.itemId === PHU)?.quantity ?? 0;
	}
	get stones() {
		return this.inventory?.currencies.spirit_stone ?? 0;
	}
	/** Daily tasks ready to claim (dock badge). */
	get claimable() {
		return this.board?.tasks.filter((t) => t.complete && !t.claimed).length ?? 0;
	}

	/** Called by the game layout with the data it loaded; fetches the rest in the background. Idempotent. */
	init(catalogue: CatalogueIndex, character: CharacterView) {
		if (this.catalogue === catalogue && this.character?.id === character.id && this.#initFrom === character) return;
		this.#initFrom = character;
		this.catalogue = catalogue;
		this.character = character;
		void this.refreshSide();
	}

	async refreshSide() {
		const [p, b, i, v] = await Promise.all([
			api.breakthroughPreview().catch(() => null),
			api.dailyTasks().catch(() => null),
			api.inventory().catch(() => null),
			api.vipStatus().catch(() => null)
		]);
		[this.preview, this.board, this.inventory, this.vip] = [p, b, i, v];
		this.ready = true;
		void this.loadRanks();
	}

	async reload() {
		this.character = await api.character();
		await this.refreshSide();
	}

	// Ranking context is decorative: failures leave it empty.
	async loadRanks() {
		const [t, r] = await Promise.all([
			api.leaderboard(0, 3).then((p) => p.entries).catch(() => [] as LeaderboardEntry[]),
			api.myRank().then((r) => r.rank).catch(() => null)
		]);
		[this.top, this.myRank] = [t, r];
	}

	async #run(action: () => Promise<string>) {
		this.busy = true;
		try {
			toast('ok', await action());
			await this.reload();
		} catch (e) {
			toast('err', errText(e));
		} finally {
			this.busy = false;
		}
	}

	claimTask = (id: string) =>
		this.#run(async () => {
			const t = await api.claimTask(id);
			fxBus.emit('reward', {
				items: t.rewards.filter((r) => r.itemId),
				currencies: t.rewards.filter((r) => r.currency),
				source: 'task',
				vipPoints: t.vipPoints
			});
			return `Đã nhận thưởng "${t.name}"${t.vipPoints ? `, +${t.vipPoints} điểm duyên` : ''}.`;
		});
	usePhu = () =>
		this.#run(async () => {
			await api.useItem(PHU, fetch, newIdempotencyKey());
			fxBus.emit('itemUsed', { itemId: PHU });
			return 'Tụ Linh Phù đã kích hoạt: lần bế quan tới được thêm 4 canh giờ.';
		});
	/** Speed item (Tụ Linh Đan): all or nothing — the server consumes it only if its full value fits today's caps. */
	useSpeed = async (itemId: string) => {
		this.busy = true;
		const key = this.#speedKeys.get(itemId) ?? newIdempotencyKey();
		this.#speedKeys.set(itemId, key);
		try {
			const from = this.character?.stageId;
			const r = await api.useItem<SpeedItemResult>(itemId, fetch, key);
			this.#speedKeys.delete(itemId);
			fxBus.emit('settled', { earnedMilli: r.earnedMilli, source: 'speed_item', stagesAdvanced: r.stagesAdvanced });
			emitStages(from, r.stagesAdvanced);
			toast('ok', `Nhận ${formatPoints(r.earnedMilli)} tu vi. Hôm nay đã dùng ${r.usesToday}/${r.maxUsesPerDay}.`);
			await this.reload();
		} catch (e) {
			if (definitive(e)) this.#speedKeys.delete(itemId);
			toast('err', errText(e));
		} finally {
			this.busy = false;
		}
	};
	/** Morning incense: once per game day; the server answers a repeat with the same offering. */
	offerIncense = () =>
		this.#run(async () => {
			const from = this.character?.stageId;
			const r = await api.incense();
			if (!r.replayed) {
				fxBus.emit('settled', { earnedMilli: r.earnedMilli, source: 'incense', stagesAdvanced: r.stagesAdvanced });
				emitStages(from, r.stagesAdvanced);
			}
			if (r.replayed) return `Hôm nay đã dâng hương — chuỗi ${r.streak.current} ngày.`;
			return `Dâng hương tĩnh tâm: chuỗi ${r.streak.current} ngày, +${formatPoints(r.earnedMilli)} tu vi${r.vipPoints ? `, +${r.vipPoints} điểm duyên` : ''}.`;
		});
	claimGift = () =>
		this.#run(async () => {
			const g = await api.claimVipGift();
			fxBus.emit('reward', { items: [], currencies: [{ currency: g.currency, amount: g.amount }], source: 'vip_gift' });
			return `Nhận quà Tiên Duyên: ${g.amount} linh thạch.`;
		});

	settle = async () => {
		this.busy = true;
		this.#settleKey ??= newIdempotencyKey();
		try {
			const from = this.character?.stageId;
			const st = await api.settle(fetch, this.#settleKey);
			this.#settleKey = null;
			// effects ("+X tu vi" float, orbs to the HUD) listen to this — server values only
			fxBus.emit('settled', { earnedMilli: st.earnedMilli, source: 'settle', stagesAdvanced: st.stagesAdvanced, hitDailyCap: st.hitDailyCap });
			emitStages(from, st.stagesAdvanced);
			const notes = [
				st.hitOfflineCap && 'đã chạm giới hạn bế quan',
				st.hitDailyCap && 'đã chạm hạn mức hôm nay',
				st.hitBottleneck && st.capMilli > 0 && 'bình cảnh đã đầy',
				st.stagesAdvanced.length && `lên ${st.stagesAdvanced.length} tầng`
			].filter(Boolean);
			toast('ok', `Thu được ${formatPoints(st.earnedMilli)} tu vi${notes.length ? ` (${notes.join(', ')})` : ''}.`);
			await this.reload();
		} catch (e) {
			if (definitive(e)) this.#settleKey = null;
			toast('err', errText(e));
		} finally {
			this.busy = false;
		}
	};

	/**
	 * Breakthrough attempt. The breakthrough mode opens at once and gathers qi while the request is in flight; the
	 * outcome plays only on the server's answer (fxBus `breakthrough`). No answer within RESPONSE_TIMEOUT_MS → a soft
	 * error with retry; the retry joins the request still in flight (same Idempotency-Key, never a second attempt) and
	 * a late answer still resolves the mode. Returns true when the attempt resolved (success or failure).
	 */
	attempt = async (): Promise<boolean> => {
		this.busy = true;
		this.#attemptKey ??= newIdempotencyKey();
		const key = this.#attemptKey;
		// Guard bound from the preview the player saw; the server refuses with 409 quote_changed if the loss grew.
		const pv = this.preview;
		const guard = pv?.tribulation && !pv.tribulation.guaranteed ? pv.tribulation.maxLossMilli : undefined;
		// …and the odds the player saw: refused if they dropped (a hộ pháp went offline, pháp bảo unequipped…).
		const seenBp = pv?.successBp;
		fxBus.emit('breakthroughStart', { kind: pv?.kind ?? 'auto', tribulation: !!pv?.tribulation });
		try {
			const r = await this.#attempt.run(() => api.breakthroughAttempt(fetch, key, guard, seenBp), RESPONSE_TIMEOUT_MS);
			await this.#attemptResolved(r);
			return true;
		} catch (e) {
			if (e instanceof TimeoutError) {
				fxBus.emit('breakthroughStalled', { message: 'Thiên đạo chưa hồi âm.' });
				this.#attempt.late((r) => void this.#attemptResolved(r));
				return false;
			}
			if (definitive(e)) this.#attemptKey = null;
			const refused = e instanceof ApiError && e.status < 500;
			if (refused || !btMode.attached) fxBus.emit('breakthroughCancelled', {});
			else fxBus.emit('breakthroughStalled', { message: errText(e) });
			if (e instanceof ApiError && (e.code === 'not_ready' || e.code === 'quote_required')) {
				this.preview = e.details as BreakthroughPreview;
				toast('warn', 'Chưa đủ điều kiện đột phá.');
			} else if (e instanceof ApiError && e.code === 'quote_changed') {
				this.preview = e.details as BreakthroughPreview;
				this.#attemptKey = null; // a new quote → a new attempt
				if (seenBp !== undefined && this.preview.successBp < seenBp) {
					toast('warn', `Tỉ lệ vượt kiếp vừa giảm còn ${formatBp(this.preview.successBp)} (đồng môn rời đi hoặc pháp bảo thay đổi). Xem lại rồi xác nhận.`);
				} else {
					toast('warn', `Tu vi vừa thay đổi: nếu thất bại sẽ mất ${formatPoints(this.preview.tribulation?.lossMilli ?? 0)} tu vi. Xem lại rồi xác nhận.`);
				}
			} else if (refused || !btMode.attached) {
				toast('err', errText(e));
			}
			return false;
		} finally {
			this.busy = false;
		}
	};

	/** The server's answer to an attempt (once per answer, even when a retry and a late callback both see it). */
	async #attemptResolved(r: BreakthroughResult) {
		if (this.#attemptsShown.has(r)) return;
		this.#attemptsShown.add(r);
		this.#attemptKey = null;
		this.busy = false;
		const mode = btMode.attached;
		fxBus.emit('breakthrough', {
			result: r.success ? 'success' : 'failure',
			kind: r.kind,
			tribulation: r.tribulation,
			lossMilli: r.lossMilli,
			from: r.from,
			to: r.to,
			failStreak: r.failStreak
		});
		if (r.success) fxBus.emit('stageUp', { from: r.from, to: r.to });
		if (!r.success) this.backlash++;
		// the mode shows the outcome with the server's text; without it (director not loaded) a toast carries it
		if (!mode) {
			if (r.success) toast('ok', 'Đột phá thành công.');
			else
				toast(
					'warn',
					r.tribulation
						? `Độ kiếp thất bại — mất ${formatPoints(r.lossMilli)} tu vi, cảnh giới giữ nguyên. Thất bại ${r.failStreak} lần liên tiếp.`
						: 'Đột phá thất bại — tu vi vẫn còn nguyên. Tâm cảnh vững hơn: lần sau tỉ lệ sẽ tăng.'
				);
		}
		await this.reload();
	}
}

export const game = new GameStore();
