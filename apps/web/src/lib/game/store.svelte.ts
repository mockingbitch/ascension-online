// Shared game state for the one-page shell. Every value comes from the server and is REPLACED by each response
// (never merged with client-computed numbers). Actions keep idempotency keys across network errors so a retry
// replays the same intent.
import { api, ApiError, newIdempotencyKey } from '$lib/api/client';
import type { CatalogueIndex } from './catalogue';
import { formatPoints } from './format';
import { toast } from './toasts.svelte';
import type {
	BreakthroughPreview,
	BreakthroughResult,
	CharacterView,
	DailyBoard,
	Inventory,
	LeaderboardEntry,
	VipStatus
} from './types';

const PHU = 'tu_linh_phu';
// 4xx = the server answered and wrote nothing; 5xx/network = unknown outcome → keep the key for the retry.
const definitive = (e: unknown) => e instanceof ApiError && e.status < 500;
const errText = (e: unknown) => (e instanceof ApiError ? e.message : String(e));

/** A "+N tu vi" float over the scene (display only). */
export interface Float {
	id: number;
	text: string;
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
	celebrate = $state<BreakthroughResult | null>(null);
	backlash = $state(0); // bump → scene plays the failed-breakthrough shake
	floats = $state<Float[]>([]);

	#settleKey: string | null = null;
	#attemptKey: string | null = null;
	#floatSeq = 0;

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

	/** Called by the game layout with the data it loaded; fetches the rest in the background. */
	init(catalogue: CatalogueIndex, character: CharacterView) {
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

	#float(text: string) {
		const id = ++this.#floatSeq;
		this.floats = [...this.floats, { id, text }];
		setTimeout(() => (this.floats = this.floats.filter((f) => f.id !== id)), 1600);
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
		this.#run(async () => `Đã nhận thưởng "${(await api.claimTask(id)).name}".`);
	usePhu = () =>
		this.#run(async () => {
			await api.useItem(PHU, fetch, newIdempotencyKey());
			return 'Tụ Linh Phù đã kích hoạt: lần bế quan tới được thêm 4 canh giờ.';
		});
	claimGift = () =>
		this.#run(async () => `Nhận quà Tiên Duyên: ${(await api.claimVipGift()).amount} linh thạch.`);

	settle = async () => {
		this.busy = true;
		this.#settleKey ??= newIdempotencyKey();
		try {
			const st = await api.settle(fetch, this.#settleKey);
			this.#settleKey = null;
			if (st.earnedMilli > 0) this.#float(`+${formatPoints(st.earnedMilli)} tu vi`);
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

	/** Returns true when the attempt resolved (success or failure) so the caller can close its window. */
	attempt = async (): Promise<boolean> => {
		this.busy = true;
		this.#attemptKey ??= newIdempotencyKey();
		// Guard bound from the preview the player saw; the server refuses with 409 quote_changed if the loss grew.
		const pv = this.preview;
		const guard = pv?.tribulation && !pv.tribulation.guaranteed ? pv.tribulation.maxLossMilli : undefined;
		try {
			const r = await api.breakthroughAttempt(fetch, this.#attemptKey, guard);
			this.#attemptKey = null;
			if (r.success) {
				this.celebrate = r;
			} else {
				this.backlash++;
				toast(
					'warn',
					r.tribulation
						? `Độ kiếp thất bại — mất ${formatPoints(r.lossMilli)} tu vi, cảnh giới giữ nguyên. Thất bại ${r.failStreak} lần liên tiếp.`
						: 'Đột phá thất bại — tu vi vẫn còn nguyên. Tâm cảnh vững hơn: lần sau tỉ lệ sẽ tăng.'
				);
			}
			await this.reload();
			return true;
		} catch (e) {
			if (definitive(e)) this.#attemptKey = null;
			if (e instanceof ApiError && (e.code === 'not_ready' || e.code === 'quote_required')) {
				this.preview = e.details as BreakthroughPreview;
				toast('warn', 'Chưa đủ điều kiện đột phá.');
			} else if (e instanceof ApiError && e.code === 'quote_changed') {
				this.preview = e.details as BreakthroughPreview;
				toast('warn', `Tu vi vừa thay đổi: nếu thất bại sẽ mất ${formatPoints(this.preview.tribulation?.lossMilli ?? 0)} tu vi. Xem lại rồi xác nhận.`);
			} else {
				toast('err', errText(e));
			}
			return false;
		} finally {
			this.busy = false;
		}
	};
}

export const game = new GameStore();
