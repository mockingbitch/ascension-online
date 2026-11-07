<!-- Bí cảnh thử luyện (docs/design/bi-canh-luat-choi.md): the daily resource mini game. The server issues the seeded
     run, replays the movement log, judges and grants the capped loot; this page only plays, records and shows the
     server's verdict and loot card. Practice mode is local and never sent. -->
<script lang="ts">
	import { ApiError, api, newIdempotencyKey } from '$lib/api/client';
	import Icon from '$lib/components/Icon.svelte';
	import ScreenFrame from '$lib/components/ScreenFrame.svelte';
	import { formatBp } from '$lib/game/format';
	import type { BiCanhParams, SecretRealmClaim, SecretRealmLoot, SecretRealmStatus, SecretRealmSubmit } from '$lib/game/types';
	import { game } from '$lib/game/store.svelte';
	import { toast } from '$lib/game/toasts.svelte';
	import BiCanhBoard from '$lib/minigames/bi-canh/BiCanhBoard.svelte';
	import type { EndReason } from '$lib/minigames/bi-canh/sim';

	let { data } = $props();

	// svelte-ignore state_referenced_locally
	let realm = $state<SecretRealmStatus | null>(data.status?.realms[0] ?? null);
	// svelte-ignore state_referenced_locally
	let recent = $state(data.status?.recent ?? []);
	type Phase = 'intro' | 'play' | 'judging' | 'verdict';
	let phase = $state<Phase>('intro');
	let run = $state<{ params: BiCanhParams; seed: number; runId: string | null; key: string; nonce: number } | null>(null);
	let claim = $state<SecretRealmClaim | null>(null);
	let local = $state<{ won: boolean; reason: EndReason; score: number; kills: number; qi: number } | null>(null);
	let verdict = $state<SecretRealmSubmit | null>(null);
	let busy = $state(false);
	let sendError = $state(false);
	let board = $state<{ giveUp: () => void }>();

	const cat = $derived(game.catalogue);
	const itemName = (id: string) => cat?.itemName(id) ?? id;
	const rarityOf = (id: string) => cat?.items.get(id)?.rarity ?? 'linh';
	const p = $derived(realm?.params);
	const seconds = (ticks: number, tickMs: number) => Math.round((ticks * tickMs) / 1000);
	const fmtTime = (iso: string | null | undefined) => (iso ? new Date(iso).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' }) : '');
	const bandName = (id: string) => realm?.bands.find((b) => b.id === id)?.name ?? id;

	const reasonText: Record<string, string> = {
		survived: 'Trụ vững tới khi kết giới khép lại.',
		fallen: 'Hộ thể đã cạn, bị đẩy ra khỏi bí cảnh.',
		gave_up: 'Bạn đã rời bí cảnh giữa chừng.',
		late: 'Phiên đã hết hạn trước khi kết quả được gửi.',
		abandoned: 'Phiên trước bị bỏ dở.',
		too_fast: 'Kết quả tới nhanh hơn thời gian ván chơi — không được ghi nhận.',
		mismatch: 'Thiên đạo phát lại từng bước nhưng không ra kết quả như đã gửi — không được ghi nhận.',
		invalid_log: 'Nhật ký di chuyển không hợp lệ — không được ghi nhận.',
		implausible_score: 'Điểm không khả thi — không được ghi nhận.',
		implausible_input: 'Đổi hướng nhanh quá sức người — không được ghi nhận.'
	};

	async function refreshStatus() {
		const st = await api.secretRealms().catch(() => null);
		if (st) {
			realm = st.realms[0] ?? realm;
			recent = st.recent;
		}
	}

	async function start() {
		if (!realm || busy) return;
		busy = true;
		try {
			const r = await api.secretRealmStart(fetch, realm.realmId);
			realm = r.status;
			if (r.resumed) toast('warn', 'Phiên đang mở được tiếp tục: thế cục như cũ, ván chơi bắt đầu lại từ đầu.');
			run = { params: r.params, seed: r.seed, runId: r.runId, key: newIdempotencyKey(), nonce: Date.now() };
			claim = null;
			verdict = null;
			phase = 'play';
		} catch (e) {
			if (e instanceof ApiError && e.details) realm = e.details as SecretRealmStatus;
			toast('err', e instanceof ApiError ? e.message : 'Không mở được bí cảnh, thử lại sau.');
		} finally {
			busy = false;
		}
	}

	function practice() {
		if (!realm) return;
		const buf = new Uint32Array(1);
		crypto.getRandomValues(buf);
		run = { params: realm.params, seed: buf[0]! || 1, runId: null, key: '', nonce: Date.now() };
		claim = null;
		verdict = null;
		phase = 'play';
	}

	function onend(c: SecretRealmClaim, l: { won: boolean; reason: EndReason; score: number; kills: number; qi: number }) {
		claim = c;
		local = l;
		if (!run?.runId) {
			phase = 'verdict';
			return;
		}
		void submit();
	}

	async function submit() {
		if (!run?.runId || !claim) return;
		phase = 'judging';
		sendError = false;
		try {
			// Same Idempotency-Key on retry: a lost response never turns into a second submission or a second loot.
			verdict = await api.secretRealmSubmit(fetch, run.runId, claim, run.key);
			realm = verdict.status;
			phase = 'verdict';
			void refreshStatus();
			if (verdict.loot) void game.refreshSide();
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
		if (confirm(run?.runId ? 'Rời bí cảnh? Phần thưởng tính theo điểm hiện có (nếu đủ thời gian tối thiểu).' : 'Dừng ván luyện tập?')) board?.giveUp();
	}

	function backToIntro() {
		phase = 'intro';
		run = null;
		void refreshStatus();
	}

	function lootLines(l: SecretRealmLoot) {
		const out: { label: string; rarity?: string; artifact?: boolean }[] = [];
		if (l.spiritStone > 0) out.push({ label: `${l.spiritStone.toLocaleString('vi-VN')} linh thạch` });
		for (const g of l.items) out.push({ label: `${g.quantity} × ${itemName(g.itemId)}`, rarity: rarityOf(g.itemId) });
		if (l.artifact) out.push({ label: itemName(l.artifact), rarity: rarityOf(l.artifact), artifact: true });
		return out;
	}
</script>

<svelte:head><title>Bí Cảnh · Tiên Đạo</title></svelte:head>

<ScreenFrame title={realm?.name ?? 'Bí Cảnh'} subtitle={phase === 'play' && run && !run.runId ? 'Luyện tập — không tính, không gửi máy chủ' : 'Thử luyện kiếm tài nguyên'} icon="portal" bleed>
	{#snippet actions()}
		{#if phase === 'play'}
			<button type="button" class="btn btn-ghost h-10 px-3 text-sm" onclick={giveUp}><Icon name="exit" size={16} /> Rời</button>
		{:else}
			<a href="/play/tui-do" class="btn btn-ghost h-10 px-3 text-sm"><Icon name="bag" size={16} /> Túi Đồ</a>
		{/if}
	{/snippet}

	{#if phase === 'play' && run}
		<div class="h-full p-3 sm:p-4">
			{#key run.nonce}
				<BiCanhBoard bind:this={board} params={run.params} seed={run.seed} practice={!run.runId} {onend} />
			{/key}
		</div>
	{:else if phase === 'judging'}
		<div class="grid h-full place-items-center p-6 text-center">
			<div>
				<div class="seal mx-auto grid size-24 place-items-center rounded-full text-gold-300" class:animate-qi-breathe={!sendError}>
					<Icon name="portal" size={36} />
				</div>
				{#if sendError}
					<p class="mt-5 text-mist-100">Chưa gửi được kết quả về máy chủ.</p>
					<p class="mt-1 text-sm text-mist-500">Ván chơi đã được lưu tạm trên trang này. Gửi lại trước khi phiên hết hạn.</p>
					<button type="button" class="btn btn-jade mt-4" onclick={() => void submit()}>Gửi lại</button>
				{:else}
					<p class="mt-5 font-display text-2xl text-gold-300">Thiên đạo đang phát lại từng bước chân…</p>
					<p class="mt-1 text-sm text-mist-500">Kết quả và phần thưởng chỉ do máy chủ phán định.</p>
				{/if}
			</div>
		</div>
	{:else if phase === 'verdict'}
		<div class="grid h-full place-items-center overflow-y-auto p-4">
			<div class="panel w-full max-w-md p-5 text-center">
				{#if !run?.runId && claim && local}
					<p class="eyebrow">Luyện tập — không tính</p>
					<p class="mt-2 font-display text-3xl font-bold" class:text-jade-300={local.won} class:text-mist-100={!local.won}>
						{local.won ? 'Trụ vững tới cuối' : 'Bị đẩy khỏi bí cảnh'}
					</p>
					<p class="mt-2 text-mist-300">
						{reasonText[local.reason] ?? ''} Được <span class="text-mist-100 tabular-nums">{local.score}</span> điểm, hạ
						<span class="text-mist-100 tabular-nums">{local.kills}</span> tà khí.
					</p>
					<p class="mt-1 text-xs text-mist-500">Ván luyện tập không gửi máy chủ nên không có phần thưởng.</p>
				{:else if verdict}
					{#if verdict.verdict === 'pass' || verdict.verdict === 'fail'}
						<div class="seal mx-auto grid size-20 place-items-center rounded-full" class:pass={verdict.verdict === 'pass'} class:text-gold-300={verdict.verdict === 'pass'} class:text-mist-300={verdict.verdict !== 'pass'}>
							<Icon name={verdict.verdict === 'pass' ? 'check' : 'portal'} size={32} />
						</div>
						<p class="mt-4 font-display text-3xl font-bold" class:text-gold-300={verdict.verdict === 'pass'} class:text-mist-100={verdict.verdict !== 'pass'}>
							{verdict.verdict === 'pass' ? 'Vượt bí cảnh' : 'Rời bí cảnh'}
						</p>
						<p class="mt-2 text-mist-300">{reasonText[verdict.reason] ?? ''}</p>
						{#if verdict.outcome}
							<p class="mt-1 text-sm text-mist-500">
								Thiên đạo ghi nhận {verdict.outcome.score} điểm · {verdict.outcome.kills} tà khí · kiếm ý {verdict.outcome.level + 1}
							</p>
						{/if}
						<div class="loot mt-4 p-3 text-left">
							{#if verdict.band && verdict.loot}
								<p class="eyebrow">Phần thưởng · {verdict.band.name}</p>
								<ul class="mt-2 space-y-1.5">
									{#each lootLines(verdict.loot) as line (line.label)}
										<li class="flex items-center gap-2" class:artifact={line.artifact}>
											<span class="gem" style:--r="var(--color-rarity-{line.rarity ?? 'linh'})" aria-hidden="true"></span>
											<span class:text-gold-300={line.artifact} class:font-semibold={line.artifact}>{line.label}</span>
											{#if line.artifact}<span class="ml-auto text-xs text-gold-300">Pháp bảo mới</span>{/if}
										</li>
									{:else}
										<li class="text-sm text-mist-500">Hôm nay đã nhận đủ phần thưởng bí cảnh.</li>
									{/each}
								</ul>
								{#if verdict.loot.duplicateOf}
									<p class="mt-2 text-xs text-mist-500">Gặp lại {itemName(verdict.loot.duplicateOf)} đã có — hóa thành linh thạch.</p>
								{/if}
								{#if verdict.loot.clipped}
									<p class="mt-2 text-xs text-mist-500">
										Một phần phần thưởng vượt hạn mức ngày nên không được nhận{verdict.loot.artifactCapped ? ' (kể cả một pháp bảo)' : ''}.
									</p>
								{/if}
								<p class="mt-2 text-xs text-mist-500">Cơ hội pháp bảo của lượt này: {formatBp(verdict.loot.artifactChanceBp)}.</p>
							{:else}
								<p class="text-sm text-mist-300">
									Chưa đủ để nhận thưởng: cần trụ ít nhất {p ? seconds(realm?.minRewardTicks ?? 0, p.tickMs) : 0} giây, đạt
									{realm?.bands[0]?.minScore ?? 0} điểm, gom {realm?.bands[0]?.minQi ?? 0} linh khí và hạ {realm?.bands[0]?.minKills ?? 0} tà khí.
								</p>
							{/if}
						</div>
						{#if verdict.vipPoints > 0}<p class="mt-2 text-sm text-mist-500">+{verdict.vipPoints} điểm duyên</p>{/if}
					{:else}
						<div class="seal mx-auto grid size-20 place-items-center rounded-full text-cinnabar-300"><Icon name="cross" size={30} /></div>
						<p class="mt-4 font-display text-3xl font-bold text-mist-100">
							{verdict.verdict === 'expired' ? 'Phiên đã hết hạn' : 'Kết quả không được ghi nhận'}
						</p>
						<p class="mt-2 text-mist-300">{reasonText[verdict.reason] ?? ''}</p>
						<p class="mt-1 text-sm text-mist-500">Lượt này đã tính vào số lượt hôm nay và không có phần thưởng.</p>
					{/if}
				{/if}
				<div class="mt-4 grid gap-2 sm:grid-cols-2">
					<button type="button" class="btn btn-ghost" onclick={backToIntro}>Về bí cảnh</button>
					<button type="button" class="btn btn-ghost" onclick={practice}><Icon name="lotus" size={16} /> Luyện tập</button>
				</div>
			</div>
		</div>
	{:else}
		<div class="grid h-full min-h-0 grid-cols-[minmax(0,1fr)] content-start overflow-y-auto lg:grid-cols-[minmax(0,1fr)_380px] lg:content-stretch">
			<section class="intro relative flex flex-col items-center justify-center gap-4 px-4 py-6 text-center">
				<div class="mist absolute inset-0" aria-hidden="true"></div>
				<div class="motif relative" aria-hidden="true"><span class="wisp"></span><span class="hero"></span><span class="wisp red"></span></div>
				<p class="relative max-w-md font-display text-2xl leading-snug text-mist-100">{realm?.summary ?? 'Kết giới bí cảnh còn đóng.'}</p>
				{#if p && realm}
					<ul class="relative max-w-md space-y-1 text-left text-sm text-mist-300">
						<li>
							• Di chuyển né tà khí trong <b class="text-mist-100">{seconds(p.timeLimitTicks, p.tickMs)} giây</b>. Phi kiếm tự tìm tà khí gần nhất; bạn chỉ cần
							đi đúng chỗ.
						</li>
						<li>• Tà khí bị hạ để lại <b class="text-spirit-cyan">linh khí</b>. Gom linh khí để kiếm ý tăng: phi kiếm nhanh hơn, rồi có kiếm trận xoay quanh.</li>
						<li>• Chạm tà khí mất một <b class="text-jade-300">hộ thể</b> ({p.player.hp} hộ thể). Hết hộ thể thì bị đẩy ra — vẫn được thưởng theo điểm.</li>
						<li>
							• Phần thưởng theo điểm do máy chủ phát lại — phải thật sự gom linh khí và hạ tà khí mới có thưởng; trụ tới cuối được thêm linh
							thạch và cơ hội pháp bảo.
						</li>
					</ul>
				{/if}
			</section>

			<aside class="order-first space-y-4 border-gold-500/15 p-4 lg:order-none lg:overflow-y-auto lg:border-l">
				{#if !realm}
					<p class="text-mist-300">Bí cảnh chưa sẵn sàng trên máy chủ này.</p>
				{:else}
					<div class="state p-3">
						{#if !realm.unlocked}
							<p class="flex items-center gap-2 font-semibold text-mist-100"><Icon name="lock" size={18} class="text-gold-500" /> Chưa đủ cảnh giới</p>
							<p class="mt-1 text-sm text-mist-300">Vẫn có thể luyện tập.</p>
						{:else if realm.openRun}
							<p class="flex items-center gap-2 font-semibold text-jade-300"><Icon name="clock" size={18} /> Phiên đang mở</p>
							<p class="mt-1 text-sm text-mist-300">Hết hạn lúc {fmtTime(realm.openRun.expiresAt)}. Tiếp tục sẽ chơi lại từ đầu với cùng thế cục.</p>
						{:else if realm.runsLeft > 0}
							<p class="flex items-center gap-2 font-semibold text-jade-300"><Icon name="portal" size={18} /> Còn {realm.runsLeft}/{realm.runsPerDay} lượt hôm nay</p>
							<p class="mt-1 text-sm text-mist-300">Mỗi lượt bắt đầu đều được tính, kể cả khi bỏ dở.</p>
						{:else}
							<p class="flex items-center gap-2 font-semibold text-mist-100"><Icon name="clock" size={18} class="text-gold-500" /> Hết lượt hôm nay</p>
							<p class="mt-1 text-sm text-mist-300">Kết giới mở lại lúc {fmtTime(realm.resetAt)}. Trong lúc chờ, có thể luyện tập.</p>
						{/if}
					</div>

					{#if realm.unlocked && (realm.openRun || realm.runsLeft > 0)}
						<button type="button" class="btn btn-gold h-12 w-full text-base" disabled={busy} onclick={() => void start()}>
							<Icon name="portal" size={18} /> {realm.openRun ? 'Tiếp tục phiên' : 'Vào bí cảnh'}
						</button>
					{/if}
					<button type="button" class="btn btn-ghost h-11 w-full" onclick={practice}><Icon name="lotus" size={16} /> Luyện tập (không tính)</button>

					<div class="state p-3 text-sm">
						<p class="eyebrow">Hôm nay đã nhận</p>
						<div class="mt-2 grid grid-cols-3 gap-2 text-center">
							<div><p class="text-lg font-semibold text-mist-100 tabular-nums">{realm.today.spiritStone}<span class="text-xs text-mist-500">/{realm.dailyCaps.spiritStone}</span></p><p class="text-xs text-mist-500">linh thạch</p></div>
							<div><p class="text-lg font-semibold text-mist-100 tabular-nums">{realm.today.materials}<span class="text-xs text-mist-500">/{realm.dailyCaps.materials}</span></p><p class="text-xs text-mist-500">nguyên liệu</p></div>
							<div><p class="text-lg font-semibold text-mist-100 tabular-nums">{realm.today.artifacts}<span class="text-xs text-mist-500">/{realm.dailyCaps.artifacts}</span></p><p class="text-xs text-mist-500">pháp bảo</p></div>
						</div>
					</div>

					<details class="state p-3 text-sm text-mist-300">
						<summary class="cursor-pointer font-semibold text-mist-100">Bảng thưởng công khai</summary>
						<table class="mt-2 w-full text-left text-xs">
							<thead class="text-mist-500"><tr><th class="py-1">Phẩm</th><th>Điểm · linh khí · tà khí</th><th>Linh thạch</th><th>Pháp bảo</th></tr></thead>
							<tbody>
								{#each realm.bands as b (b.id)}
									{@const l = realm.loot[b.id]}
									<tr class="border-t border-ink-700">
										<td class="py-1">{b.name}</td>
										<td class="tabular-nums">≥ {b.minScore} · {b.minQi} · {b.minKills}</td>
										<td class="tabular-nums">{l?.spiritStone.min}–{l?.spiritStone.max}</td>
										<td class="tabular-nums">{formatBp(l?.artifactChanceBp ?? 0)}</td>
									</tr>
								{/each}
							</tbody>
						</table>
						<p class="mt-2 text-xs">Trụ tới cuối: +{realm.survived.spiritStone} linh thạch, +{formatBp(realm.survived.artifactChanceBp)} cơ hội pháp bảo.</p>
						<p class="mt-2 text-xs text-mist-500">Pháp bảo có thể rơi:</p>
						<ul class="mt-1 space-y-0.5 text-xs">
							{#each realm.artifacts as a (a.itemId)}
								<li class="flex items-center gap-2">
									<span class="gem" style:--r="var(--color-rarity-{rarityOf(a.itemId)})" aria-hidden="true"></span>{itemName(a.itemId)}
									<span class="ml-auto text-mist-500">lôi kiếp +{formatBp(cat?.items.get(a.itemId)?.tribulationBonusBp ?? 0)}</span>
								</li>
							{/each}
						</ul>
						<p class="mt-2 text-xs text-mist-500">Pháp bảo chỉ rơi từ bí cảnh, không bán, không trao đổi. Tiên Duyên không ảnh hưởng lượt hay phần thưởng.</p>
					</details>

					{#if recent.length > 0}
						<div class="space-y-1 text-xs text-mist-500">
							<p class="eyebrow">Lượt gần đây</p>
							{#each recent as r (r.runId)}
								<p class="flex gap-2">
									<span>{fmtTime(r.issuedAt)}</span>
									<span class="text-mist-300">{r.band ? bandName(r.band) : r.verdict === 'rejected' ? 'không ghi nhận' : r.verdict === 'expired' ? 'hết hạn' : 'không thưởng'}</span>
									{#if r.loot}<span class="ml-auto">+{r.loot.spiritStone} LT{r.loot.artifact ? ` · ${itemName(r.loot.artifact)}` : ''}</span>{/if}
								</p>
							{/each}
						</div>
					{/if}
				{/if}
			</aside>
		</div>
	{/if}
</ScreenFrame>

<style>
	.mist {
		background:
			radial-gradient(ellipse at 50% 30%, color-mix(in oklab, var(--color-spirit-violet) 12%, transparent), transparent 60%),
			radial-gradient(ellipse at 50% 85%, color-mix(in oklab, var(--color-spirit-cyan) 8%, transparent), transparent 60%);
	}
	.motif {
		display: flex;
		align-items: center;
		gap: 16px;
	}
	.wisp {
		width: 26px;
		height: 26px;
		border-radius: 45% 55% 50% 50%;
		background: var(--color-spirit-violet);
		opacity: 0.8;
	}
	.wisp.red {
		width: 18px;
		height: 18px;
		background: var(--color-cinnabar-300);
	}
	.hero {
		width: 22px;
		height: 30px;
		clip-path: polygon(50% 0, 100% 100%, 0 100%);
		background: var(--color-jade-300);
		box-shadow: 0 0 18px -2px var(--color-gold-300);
	}
	.state,
	.loot {
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 25%, transparent);
		background: color-mix(in oklab, var(--color-ink-950) 50%, transparent);
	}
	.gem {
		display: inline-block;
		width: 10px;
		height: 10px;
		transform: rotate(45deg);
		background: var(--r);
		flex-shrink: 0;
	}
	.artifact {
		padding: 4px 6px;
		margin: 0 -6px;
		background: color-mix(in oklab, var(--color-gold-500) 10%, transparent);
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
