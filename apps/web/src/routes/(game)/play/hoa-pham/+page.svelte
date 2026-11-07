<!-- Hóa Phàm: the trial guarding Nguyên Anh → Hóa Thần (docs/design/hoa-pham-luat-choi.md).
     The server issues the seeded run and judges the submission by replaying the stroke log; this page only plays,
     records and shows the server's verdict. Practice mode is local and never sent. -->
<script lang="ts">
	import { ApiError, api, newIdempotencyKey } from '$lib/api/client';
	import Icon from '$lib/components/Icon.svelte';
	import RealmBadge from '$lib/components/RealmBadge.svelte';
	import ScreenFrame from '$lib/components/ScreenFrame.svelte';
	import type { HoaPhamParams, TrialClaim, TrialKindStatus, TrialRunStart, TrialSubmitResult } from '$lib/game/types';
	import { game } from '$lib/game/store.svelte';
	import { toast } from '$lib/game/toasts.svelte';
	import HoaPhamBoard from '$lib/minigames/hoa-pham/HoaPhamBoard.svelte';
	import type { EndReason } from '$lib/minigames/hoa-pham/sim';

	let { data } = $props();

	const KIND = 'hoa_pham';
	// svelte-ignore state_referenced_locally
	let kind = $state<TrialKindStatus | null>(data.status?.kinds.find((k) => k.kind === KIND) ?? null);
	type Phase = 'intro' | 'play' | 'judging' | 'verdict';
	let phase = $state<Phase>('intro');
	let run = $state<{ params: HoaPhamParams; seed: number; runId: string | null; key: string; nonce: number } | null>(null);
	let claim = $state<TrialClaim | null>(null);
	let local = $state<{ won: boolean; reason: EndReason; score: number; cuts: number } | null>(null);
	let verdict = $state<TrialSubmitResult | null>(null);
	let busy = $state(false);
	let sendError = $state(false);
	let board = $state<{ giveUp: () => void }>();

	const cat = $derived(game.catalogue!);
	const fromRealm = $derived(kind?.fromRealmId ? cat.realms.get(kind.fromRealmId) : undefined);
	const toRealm = $derived(kind?.toRealmId ? cat.realms.get(kind.toRealmId) : undefined);
	const p = $derived(kind?.params as HoaPhamParams | undefined);
	const seconds = (ticks: number, tickMs: number) => Math.round((ticks * tickMs) / 1000);
	const fmtTime = (iso: string | null | undefined, withDate = false) =>
		iso
			? new Date(iso).toLocaleString('vi-VN', withDate ? { hour: '2-digit', minute: '2-digit', day: '2-digit', month: '2-digit' } : { hour: '2-digit', minute: '2-digit' })
			: '';

	const reasonText: Record<string, string> = {
		cleared: 'Trần duyên đã đoạn đủ, tâm trở về giản phác.',
		lanterns_out: 'Đèn chân tâm đã tắt hết.',
		time_up: 'Hết thời gian thử luyện mà trần duyên chưa đoạn đủ.',
		gave_up: 'Bạn đã dừng giữa chừng.',
		late: 'Phiên đã hết hạn trước khi kết quả được gửi.',
		abandoned: 'Phiên trước bị bỏ dở.',
		too_fast: 'Kết quả tới nhanh hơn thời gian ván chơi — không được ghi nhận.',
		mismatch: 'Thiên đạo phát lại từng nét chém nhưng không ra kết quả như đã gửi — không được ghi nhận.',
		invalid_log: 'Nhật ký nét chém không hợp lệ — không được ghi nhận.',
		implausible_score: 'Điểm không khả thi — không được ghi nhận.',
		implausible_input: 'Nét chém nhanh quá sức người — không được ghi nhận.'
	};

	async function refreshStatus() {
		const st = await api.trials().catch(() => null);
		kind = st?.kinds.find((k) => k.kind === KIND) ?? kind;
	}

	async function start() {
		if (!kind || busy) return;
		busy = true;
		try {
			const r: TrialRunStart = await api.trialStart(fetch, kind.kind);
			kind = r.status;
			if (r.resumed) toast('warn', 'Phiên đang mở được tiếp tục: thế cục như cũ, ván chơi bắt đầu lại từ đầu.');
			run = { params: r.params as HoaPhamParams, seed: r.seed, runId: r.runId, key: newIdempotencyKey(), nonce: Date.now() };
			claim = null;
			verdict = null;
			phase = 'play';
		} catch (e) {
			if (e instanceof ApiError && e.details) kind = e.details as TrialKindStatus;
			toast('err', e instanceof ApiError ? e.message : 'Không mở được thử luyện, thử lại sau.');
		} finally {
			busy = false;
		}
	}

	function practice() {
		if (!kind) return;
		const buf = new Uint32Array(1);
		crypto.getRandomValues(buf);
		run = { params: kind.params as HoaPhamParams, seed: buf[0]! || 1, runId: null, key: '', nonce: Date.now() };
		claim = null;
		verdict = null;
		phase = 'play';
	}

	function onend(c: TrialClaim, l: { won: boolean; reason: EndReason; score: number; cuts: number }) {
		claim = c;
		local = l;
		if (!run?.runId) {
			phase = 'verdict'; // practice: nothing to send
			return;
		}
		void submit();
	}

	async function submit() {
		if (!run?.runId || !claim) return;
		phase = 'judging';
		sendError = false;
		try {
			// Same Idempotency-Key on retry: a lost response never turns into a second submission.
			verdict = await api.trialSubmit(fetch, run.runId, claim, run.key);
			kind = verdict.status;
			phase = 'verdict';
			if (verdict.verdict === 'pass') void game.refreshSide(); // the breakthrough preview now shows the pass
		} catch (e) {
			if (e instanceof ApiError && e.code === 'already_submitted') {
				await refreshStatus();
				phase = 'intro';
				toast('warn', 'Phiên này đã được phán định trước đó.');
				return;
			}
			sendError = true;
		}
	}

	function giveUp() {
		if (confirm(run?.runId ? 'Dừng thử luyện? Lần này sẽ tính là chưa qua.' : 'Dừng ván luyện tập?')) board?.giveUp();
	}

	function backToIntro() {
		phase = 'intro';
		run = null;
		void refreshStatus();
	}
</script>

<svelte:head><title>Hóa Phàm · Tiên Đạo</title></svelte:head>

<ScreenFrame title="Hóa Phàm" subtitle={phase === 'play' && run && !run.runId ? 'Luyện tập — không tính, không gửi máy chủ' : toRealm ? `Đoạn trần duyên trước khi lên ${toRealm.name}` : 'Đoạn trần duyên'} icon="lotus" bleed>
	{#snippet actions()}
		{#if phase === 'play'}
			<button type="button" class="btn btn-ghost h-10 px-3 text-sm" onclick={giveUp}><Icon name="exit" size={16} /> Dừng</button>
		{:else}
			<a href="/play/dot-pha" class="btn btn-ghost h-10 px-3 text-sm"><Icon name="bolt" size={16} /> Đột Phá</a>
		{/if}
	{/snippet}

	{#if phase === 'play' && run}
		<div class="h-full p-3 sm:p-4">
			{#key run.nonce}
				<HoaPhamBoard bind:this={board} params={run.params} seed={run.seed} practice={!run.runId} {onend} />
			{/key}
		</div>
	{:else if phase === 'judging'}
		<div class="grid h-full place-items-center p-6 text-center">
			<div>
				<div class="seal mx-auto grid size-24 place-items-center rounded-full text-gold-300" class:animate-qi-breathe={!sendError}>
					<Icon name="lotus" size={36} />
				</div>
				{#if sendError}
					<p class="mt-5 text-mist-100">Chưa gửi được kết quả về máy chủ.</p>
					<p class="mt-1 text-sm text-mist-500">Ván chơi đã được lưu tạm trên trang này. Gửi lại trước khi phiên hết hạn.</p>
					<button type="button" class="btn btn-jade mt-4" onclick={() => void submit()}>Gửi lại</button>
				{:else}
					<p class="mt-5 font-display text-2xl text-gold-300">Thiên đạo đang phát lại từng nét chém…</p>
					<p class="mt-1 text-sm text-mist-500">Kết quả chỉ do máy chủ phán định.</p>
				{/if}
			</div>
		</div>
	{:else if phase === 'verdict'}
		<div class="grid h-full place-items-center overflow-y-auto p-4">
			<div class="panel w-full max-w-md p-5 text-center">
				{#if !run?.runId && claim && local}
					<p class="eyebrow">Luyện tập — không tính</p>
					<p class="mt-2 font-display text-3xl font-bold" class:text-jade-300={local.won} class:text-mist-100={!local.won}>
						{local.won ? 'Tâm đã giản phác' : 'Trần duyên còn vương'}
					</p>
					<p class="mt-2 text-mist-300">
						{reasonText[local.reason] ?? ''} Được <span class="text-mist-100 tabular-nums">{local.score}</span> điểm trần duyên, đoạn
						<span class="text-mist-100 tabular-nums">{local.cuts}</span> nút.
					</p>
					<p class="mt-1 text-xs text-mist-500">Ván luyện tập không gửi máy chủ nên không mở cửa đột phá.</p>
				{:else if verdict}
					{#if verdict.verdict === 'pass'}
						<div class="seal pass mx-auto grid size-20 place-items-center rounded-full text-gold-300"><Icon name="check" size={34} /></div>
						<p class="mt-4 font-display text-3xl font-bold text-gold-300">Hóa phàm đã thành</p>
						<p class="mt-2 text-mist-300">
							{reasonText[verdict.reason] ?? ''} Ấn chứng có hiệu lực tới
							<span class="text-mist-100">{fmtTime(verdict.passValidUntil, true)}</span> và được giữ qua các lần đột phá thất bại.
						</p>
						{#if verdict.vipPoints > 0}<p class="mt-1 text-sm text-mist-500">+{verdict.vipPoints} điểm duyên</p>{/if}
						<a href="/play/dot-pha" class="btn btn-gold mt-5 h-12 w-full text-base"><Icon name="bolt" size={18} /> Tới Đột Phá</a>
					{:else}
						<div class="seal mx-auto grid size-20 place-items-center rounded-full text-cinnabar-300"><Icon name="cross" size={30} /></div>
						<p class="mt-4 font-display text-3xl font-bold text-mist-100">
							{verdict.verdict === 'fail' ? 'Trần duyên còn vương' : verdict.verdict === 'expired' ? 'Phiên đã hết hạn' : 'Kết quả không được ghi nhận'}
						</p>
						<p class="mt-2 text-mist-300">{reasonText[verdict.reason] ?? ''}</p>
						{#if verdict.outcome}
							<p class="mt-1 text-sm text-mist-500">
								Thiên đạo ghi nhận {verdict.outcome.score}/{run?.params.passScore} điểm trần duyên, đoạn {verdict.outcome.lines} nút.
							</p>
						{/if}
						<p class="mt-3 text-sm text-mist-300">
							Không mất tu vi. Tĩnh dưỡng rồi thử lại lúc <span class="text-mist-100">{fmtTime(verdict.cooldownUntil)}</span>.
						</p>
					{/if}
				{/if}
				<div class="mt-4 grid gap-2 sm:grid-cols-2">
					<button type="button" class="btn btn-ghost" onclick={backToIntro}>Về thử luyện</button>
					<button type="button" class="btn btn-ghost" onclick={practice}><Icon name="lotus" size={16} /> Luyện tập</button>
				</div>
			</div>
		</div>
	{:else}
		<div class="grid h-full min-h-0 grid-cols-[minmax(0,1fr)] content-start overflow-y-auto lg:grid-cols-[minmax(0,1fr)_380px] lg:content-stretch">
			<section class="intro relative flex flex-col items-center justify-center gap-4 px-4 py-6 text-center">
				<div class="mist absolute inset-0" aria-hidden="true"></div>
				<div class="relative flex items-center gap-3">
					{#if fromRealm}<RealmBadge name={fromRealm.name} aura={fromRealm.visual.aura} size="sm" />{/if}
					<Icon name="arrow" size={24} class="text-gold-300" />
					{#if toRealm}<RealmBadge name={toRealm.name} aura={toRealm.visual.aura} size="sm" />{/if}
				</div>
				<div class="motif relative" aria-hidden="true">
					<span class="knot"></span><span class="lamp"></span><span class="knot small"></span>
				</div>
				<p class="relative max-w-md font-display text-2xl leading-snug text-mist-100">{kind?.summary ?? 'Trần duyên chưa hiện.'}</p>
				{#if p}
					<ul class="relative max-w-md space-y-1 text-left text-sm text-mist-300">
						<li>
							• Vuốt (hoặc kéo chuột) qua các nút trần duyên để chém. Đạt <b class="text-mist-100">{p.passScore}</b> điểm trong
							<b class="text-mist-100">{seconds(p.timeLimitTicks, p.tickMs)} giây</b> và giữ tâm ít nhất
							<b class="text-mist-100">{seconds(p.minPassTicks, p.tickMs)} giây</b> để vượt qua.
						</li>
						<li>• Nút thường 1 điểm, tơ vương nhỏ 2 điểm, lưới nghiệp 3 điểm nhưng cần hai nét. Một nét đoạn nhiều nút được cộng thêm.</li>
						<li>• <b class="text-gold-300">Đèn chân tâm</b> là tâm thật — đừng chém. Chém tắt {p.lanterns} đèn là thua.</li>
						<li>• Đoạn liền {p.calm.chargeMax} nút không để sót thì vào <b class="text-spirit-cyan">tâm tĩnh</b>: vạn vật chậm lại {seconds(p.calm.durationTicks, p.tickMs)} giây.</li>
						<li>• Thua không mất tu vi, chỉ phải tĩnh dưỡng một lúc.</li>
					</ul>
				{/if}
			</section>

			<aside class="order-first space-y-4 border-gold-500/15 p-4 lg:order-none lg:overflow-y-auto lg:border-l">
				{#if !kind}
					<p class="text-mist-300">Thử luyện chưa sẵn sàng trên máy chủ này.</p>
				{:else}
					<div class="state p-3">
						{#if kind.state === 'passed' && kind.pass}
							<p class="flex items-center gap-2 font-semibold text-gold-300"><Icon name="check" size={18} /> Đã hóa phàm</p>
							<p class="mt-1 text-sm text-mist-300">Ấn chứng còn hiệu lực tới {fmtTime(kind.pass.validUntil, true)}. Hãy tới Đột Phá khi tu vi viên mãn.</p>
						{:else if kind.state === 'in_progress' && kind.openRun}
							<p class="flex items-center gap-2 font-semibold text-jade-300"><Icon name="clock" size={18} /> Phiên đang mở</p>
							<p class="mt-1 text-sm text-mist-300">Hết hạn lúc {fmtTime(kind.openRun.expiresAt)}. Tiếp tục sẽ chơi lại từ đầu với cùng thế cục.</p>
						{:else if kind.state === 'cooldown'}
							<p class="flex items-center gap-2 font-semibold text-mist-100"><Icon name="clock" size={18} class="text-gold-500" /> Tâm cảnh còn dao động</p>
							<p class="mt-1 text-sm text-mist-300">Thử lại lúc {fmtTime(kind.cooldownUntil)}. Trong lúc chờ, có thể luyện tập.</p>
						{:else if kind.state === 'locked'}
							<p class="flex items-center gap-2 font-semibold text-mist-100"><Icon name="lock" size={18} class="text-gold-500" /> Chưa tới lúc</p>
							<p class="mt-1 text-sm text-mist-300">
								Trần duyên chỉ hiện khi đứng trước bình cảnh {fromRealm?.name ?? ''}{kind.opensAt === 'last_stage' ? ' (tầng cuối)' : ''}. Bạn vẫn có thể luyện tập.
							</p>
						{:else}
							<p class="flex items-center gap-2 font-semibold text-jade-300"><Icon name="spark" size={18} /> Có thể thử luyện</p>
							<p class="mt-1 text-sm text-mist-300">Qua ải này mới có thể đột phá lên {toRealm?.name ?? 'cảnh giới kế tiếp'}.</p>
						{/if}
					</div>

					{#if kind.state === 'available' || kind.state === 'in_progress'}
						<button type="button" class="btn btn-gold h-12 w-full text-base" disabled={busy} onclick={() => void start()}>
							<Icon name="lotus" size={18} /> {kind.state === 'in_progress' ? 'Tiếp tục phiên' : 'Bắt đầu thử luyện'}
						</button>
					{/if}
					<button type="button" class="btn btn-ghost h-11 w-full" onclick={practice}><Icon name="lotus" size={16} /> Luyện tập (không tính)</button>

					<div class="space-y-1 text-xs text-mist-500">
						<p>Thua, bỏ dở hoặc kết quả không hợp lệ: chờ {Math.round(kind.loseCooldownSeconds / 60)} phút rồi thử lại.</p>
						<p>Ấn chứng hóa phàm giữ {Math.round(kind.passValidSeconds / 86400)} ngày, không mất khi đột phá thất bại, dùng hết khi đột phá thành công.</p>
						<p>Máy chủ phát lại từng nét chém để phán định; tạm dừng không dừng giờ của phiên.</p>
					</div>
				{/if}
			</aside>
		</div>
	{/if}
</ScreenFrame>

<style>
	.mist {
		background:
			radial-gradient(ellipse at 50% 30%, color-mix(in oklab, var(--color-gold-500) 10%, transparent), transparent 60%),
			radial-gradient(ellipse at 50% 85%, color-mix(in oklab, var(--color-cinnabar-300) 10%, transparent), transparent 60%);
	}
	.motif {
		display: flex;
		align-items: center;
		gap: 14px;
	}
	.knot {
		width: 30px;
		height: 30px;
		border-radius: 50%;
		border: 2px solid var(--color-cinnabar-300);
		box-shadow:
			inset 4px -3px 0 -1px color-mix(in oklab, var(--color-cinnabar-300) 70%, transparent),
			inset -5px 4px 0 -2px color-mix(in oklab, var(--color-cinnabar-300) 60%, transparent);
		transform: rotate(-20deg) scaleY(0.8);
	}
	.knot.small {
		width: 20px;
		height: 20px;
		border-color: var(--color-jade-300);
		box-shadow: inset 3px -2px 0 -1px color-mix(in oklab, var(--color-jade-300) 70%, transparent);
	}
	.lamp {
		width: 22px;
		height: 30px;
		border-radius: 45%;
		background: radial-gradient(circle at 50% 60%, var(--color-gold-300), var(--color-gold-500) 70%);
		box-shadow: 0 0 18px -2px color-mix(in oklab, var(--color-gold-300) 70%, transparent);
		transform: translateY(-8px);
	}
	.state {
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 25%, transparent);
		background: color-mix(in oklab, var(--color-ink-950) 50%, transparent);
	}
	.seal {
		background: radial-gradient(circle, color-mix(in oklab, var(--color-gold-500) 14%, var(--color-ink-800)), var(--color-ink-950));
		box-shadow:
			0 0 0 1px var(--color-gold-500),
			0 0 0 6px var(--color-ink-950),
			0 0 0 7px color-mix(in oklab, var(--color-gold-500) 40%, transparent);
	}
	.seal.pass {
		box-shadow:
			0 0 0 1px var(--color-gold-300),
			0 0 0 6px var(--color-ink-950),
			0 0 30px -4px color-mix(in oklab, var(--color-gold-300) 50%, transparent);
	}
</style>
