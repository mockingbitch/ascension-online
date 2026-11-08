<!-- Hóa Phàm: the trial guarding Nguyên Anh → Hóa Thần (docs/design/hoa-pham-luat-choi.md).
     The server issues the seeded run and judges the submission by replaying the stroke log; this page only plays,
     records and shows the server's verdict. Practice mode is local and never sent. -->
<script lang="ts">
	import { onDestroy, tick } from 'svelte';
	import { ApiError, api, newIdempotencyKey } from '$lib/api/client';
	import Icon from '$lib/components/Icon.svelte';
	import RealmBadge from '$lib/components/RealmBadge.svelte';
	import { leaveRoute } from '$lib/game/windows';
	import type { HoaPhamParams, TrialClaim, TrialKindStatus, TrialRunStart, TrialSubmitResult } from '$lib/game/types';
	import { game } from '$lib/game/store.svelte';
	import { InFlight, TimeoutError } from '$lib/game/inflight';
	import { fxBus } from '$lib/engine/fx/bus';
	import { countdownMs, RESPONSE_TIMEOUT_MS } from '$lib/engine/mode/machine';
	import { ModeController } from '$lib/engine/mode/mode.svelte';
	import ModeResult from '$lib/engine/mode/ModeResult.svelte';
	import ModeShell from '$lib/engine/mode/ModeShell.svelte';
	import { settings } from '$lib/engine/settings.svelte';
	import { toast } from '$lib/game/toasts.svelte';
	import HoaPhamBoard from '$lib/minigames/hoa-pham/HoaPhamBoard.svelte';
	import type { EndReason } from '$lib/minigames/hoa-pham/sim';

	let { data } = $props();

	const KIND = 'hoa_pham';

	// svelte-ignore state_referenced_locally
	let kind = $state<TrialKindStatus | null>(data.status?.kinds.find((k) => k.kind === KIND) ?? null);
	let run = $state<{ params: HoaPhamParams; seed: number; runId: string | null; key: string; nonce: number } | null>(null);
	let claim = $state<TrialClaim | null>(null);
	let local = $state<{ won: boolean; reason: EndReason; score: number; cuts: number } | null>(null);
	let verdict = $state<TrialSubmitResult | null>(null);
	let busy = $state(false);
	let board = $state<{ giveUp: () => void }>();
	const submitting = new InFlight<TrialSubmitResult>();

	// Full-screen mode (lobby → 3-2-1 → play → the server judges → result); Esc/Back pause first, then confirm.
	const mode = new ModeController({ exit: leaveRoute, giveUp: () => board?.giveUp() });
	onDestroy(() => mode.dispose());

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

	function begin(r: NonNullable<typeof run>) {
		run = r;
		claim = null;
		verdict = null;
		mode.send({ type: 'start', countdownMs: countdownMs(settings.fx) });
	}

	async function start() {
		if (!kind || busy) return;
		busy = true;
		try {
			const r: TrialRunStart = await api.trialStart(fetch, kind.kind);
			kind = r.status;
			if (r.resumed) toast('warn', 'Phiên đang mở được tiếp tục: thế cục như cũ, ván chơi bắt đầu lại từ đầu.');
			begin({ params: r.params as HoaPhamParams, seed: r.seed, runId: r.runId, key: newIdempotencyKey(), nonce: Date.now() });
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
		begin({ params: kind.params as HoaPhamParams, seed: buf[0]! || 1, runId: null, key: '', nonce: Date.now() });
	}

	function onend(c: TrialClaim, l: { won: boolean; reason: EndReason; score: number; cuts: number }) {
		claim = c;
		local = l;
		mode.send({ type: 'ended', local: !run?.runId }); // practice: nothing to send
		if (run?.runId) void submit();
	}

	/** The server's verdict (once per run, even if a retry and a late answer both deliver it). */
	async function apply(v: TrialSubmitResult, key: string) {
		if (!run || run.key !== key || verdict) return;
		verdict = v;
		kind = v.status;
		mode.send({ type: 'verdict' });
		await tick(); // the result screen is on screen before its effects start
		if (v.verdict === 'pass' || v.verdict === 'fail') fxBus.emit('trialResult', { verdict: v.verdict });
		if (v.verdict === 'pass') void game.refreshSide(); // the breakthrough preview now shows the pass
	}

	async function submit() {
		if (!run?.runId || !claim) return;
		const { runId, key } = run;
		const c = claim;
		if (mode.phase === 'error') mode.send({ type: 'retry' });
		try {
			// Same Idempotency-Key on retry (and a retry joins a request still in flight): never a second submission.
			const v = await submitting.run(() => api.trialSubmit(fetch, runId, c, key), RESPONSE_TIMEOUT_MS);
			await apply(v, key);
		} catch (e) {
			if (e instanceof TimeoutError) {
				mode.send({ type: 'fail' });
				submitting.late((v) => void apply(v, key));
				return;
			}
			if (e instanceof ApiError && e.code === 'already_submitted') {
				await refreshStatus();
				backToIntro();
				toast('warn', 'Phiên này đã được phán định trước đó.');
				return;
			}
			mode.send({ type: 'fail' });
		}
	}

	function backToIntro() {
		mode.send({ type: 'reset' });
		run = null;
		void refreshStatus();
	}
	const replay = () => (run && !run.runId ? practice() : backToIntro());
</script>

<svelte:head><title>Hóa Phàm · Tiên Đạo</title></svelte:head>

<ModeShell
	{mode}
	title="Hóa Phàm"
	subtitle={run && !run.runId && mode.phase !== 'lobby' ? 'Luyện tập — không tính, không gửi máy chủ' : toRealm ? `Đoạn trần duyên trước khi lên ${toRealm.name}` : 'Đoạn trần duyên'}
	icon="lotus"
	orientation="portrait"
	waitingText="Thiên đạo đang phát lại từng nét chém…"
	confirmText={run?.runId ? 'Dừng giữa chừng: lần này sẽ tính là chưa qua.' : 'Dừng ván luyện tập?'}
	onretry={() => void submit()}
>
	{#if (mode.phase === 'countdown' || mode.phase === 'play') && run}
		<div class="h-full p-3 sm:p-4">
			{#key run.nonce}
				<HoaPhamBoard
					bind:this={board}
					bind:paused={() => mode.paused, (v) => mode.send({ type: v ? 'pause' : 'resume' })}
					hold={mode.phase === 'countdown'}
					countdownMs={0}
					params={run.params}
					seed={run.seed}
					practice={!run.runId}
					{onend}
				/>
			{/key}
		</div>
	{:else if mode.phase === 'result'}
		{#if !run?.runId && claim && local}
			<ModeResult
				verdict="practice"
				heading={local.won ? 'Tâm đã giản phác' : 'Trần duyên còn vương'}
				reason={reasonText[local.reason] ?? ''}
				stats={[
					{ label: 'Điểm trần duyên', value: `${local.score}/${run?.params.passScore ?? ''}` },
					{ label: 'Nút đã đoạn', value: String(local.cuts) }
				]}
				replayLabel="Luyện lại"
				onreplay={replay}
				onback={leaveRoute}
			>
				<p class="text-xs text-mist-500">Ván luyện tập không gửi máy chủ nên không mở cửa đột phá.</p>
			</ModeResult>
		{:else if verdict}
			<ModeResult
				verdict={verdict.verdict === 'pass' ? 'pass' : verdict.verdict === 'fail' ? 'fail' : 'void'}
				heading={verdict.verdict === 'pass'
					? 'Hóa phàm đã thành'
					: verdict.verdict === 'fail'
						? 'Trần duyên còn vương'
						: verdict.verdict === 'expired'
							? 'Phiên đã hết hạn'
							: 'Kết quả không được ghi nhận'}
				reason={reasonText[verdict.reason] ?? ''}
				stats={verdict.outcome
					? [
							{ label: 'Điểm trần duyên', value: `${verdict.outcome.score}/${run?.params.passScore ?? ''}` },
							{ label: 'Nút đã đoạn', value: String(verdict.outcome.lines) }
						]
					: []}
				onreplay={replay}
				onback={leaveRoute}
			>
				{#if verdict.verdict === 'pass'}
					<p>Ấn chứng có hiệu lực tới <span class="text-mist-100">{fmtTime(verdict.passValidUntil, true)}</span> và được giữ qua các lần đột phá thất bại.</p>
					{#if verdict.vipPoints > 0}<p class="text-mist-500">+{verdict.vipPoints} điểm duyên</p>{/if}
				{:else}
					<p>Không mất tu vi. Tĩnh dưỡng rồi thử lại lúc <span class="text-mist-100">{fmtTime(verdict.cooldownUntil)}</span>.</p>
				{/if}
				{#snippet actions()}
					{#if verdict?.verdict === 'pass'}
						<a href="/play/dot-pha" class="btn btn-gold h-12 w-full text-base"><Icon name="bolt" size={18} /> Tới Đột Phá</a>
					{/if}
				{/snippet}
			</ModeResult>
		{/if}
	{:else if mode.phase === 'lobby'}
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
</ModeShell>

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
</style>
