<!--
	Công trình thế lực (GET /factions/works, POST /factions/donate): members donate spirit stones to build their
	faction's shared works; finished stages unlock PURELY VISUAL sơn môn decorations. Donating adds no merit, rank,
	điểm duyên or tu vi. Every number comes from the server; bars are display only.
-->
<script lang="ts">
	import { fxBus } from '$lib/engine/fx/bus';
	import { api, ApiError, newIdempotencyKey } from '$lib/api/client';
	import { factionOf } from '$lib/game/faction.svelte';
	import { formatQi, progressRatio } from '$lib/game/format';
	import { game } from '$lib/game/store.svelte';
	import { toast } from '$lib/game/toasts.svelte';
	import type { FactionWorksView } from '$lib/game/types';
	import CosmeticArt from './CosmeticArt.svelte';
	import GameWindow from './GameWindow.svelte';
	import Icon from './Icon.svelte';

	interface Props {
		/** compact = overview of every faction only (non-members) */
		compact?: boolean;
	}
	let { compact = false }: Props = $props();

	let view = $state<FactionWorksView | null>(null);
	let failed = $state(false);
	let open = $state(false);
	let amount = $state(0);
	let busy = $state(false);
	let key: string | null = null; // kept across network errors so a retry replays the same donation

	$effect(() => {
		api
			.factionWorks()
			.then((v) => (view = v))
			.catch(() => (failed = true));
	});

	const mine = $derived(view?.mine ?? null);
	const work = $derived(view && mine ? view.works[mine.progress.workIndex] : undefined);
	const capLeft = $derived(view ? Math.max(0, view.settings.dailyCapPerCharacter - view.todayDonated) : 0);
	const most = $derived(view ? Math.min(capLeft, view.balance) : 0);
	const presets = $derived(view ? [view.settings.minAmount * 10, view.settings.dailyCapPerCharacter / 2, view.settings.dailyCapPerCharacter] : []);
	const decoName = (slug: string) => {
		for (const w of view?.works ?? []) for (const s of w.stages) if (s.decoration.slug === slug) return s.decoration.name;
		return slug;
	};
	const totalOf = (id: string) => view?.factions.find((x) => x.factionId === id)?.donatedTotal ?? 0;
	const others = $derived(
		(view?.factions ?? []).filter((x) => x.donatedTotal > 0).sort((a, b) => b.donatedTotal - a.donatedTotal)
	);

	function start() {
		amount = Math.min(most, Math.max(view?.settings.minAmount ?? 0, presets[1] ?? 0));
		key = null;
		open = true;
	}

	async function donate() {
		busy = true;
		key ??= newIdempotencyKey();
		try {
			const r = await api.factionDonate(amount, key);
			key = null;
			const before = view?.mine?.progress.unlocked.length ?? 0;
			view = r.state;
			const after = r.state.mine?.progress.unlocked.length ?? 0;
			if (!r.replayed && r.state.mine && after > before) fxBus.emit('worksLevelUp', { factionId: r.state.mine.factionId, unlocked: after });
			toast('ok', `Đã cúng góp ${r.amount.toLocaleString('vi-VN')} linh thạch cho công trình.`);
			open = false;
			await game.reload();
		} catch (e) {
			if (e instanceof ApiError && e.status < 500) key = null;
			toast('err', e instanceof ApiError ? e.message : String(e));
			api.factionWorks().then((v) => (view = v)).catch(() => {});
		} finally {
			busy = false;
		}
	}
</script>

{#if failed}
	<p class="text-sm text-mist-500">Chưa xem được công trình — thử lại sau.</p>
{:else if !view}
	<p class="text-sm text-mist-500">Đang xem công trình…</p>
{:else}
	{@const v = view}
	{#if mine && !compact}
		<section>
			<h2 class="panel-title">Công trình sơn môn</h2>
			{#if work}
				{@const done = mine.progress.workDonated}
				<div class="work mt-3 p-4">
					<p class="eyebrow">Công trình {mine.progress.workIndex + 1}/{v.works.length}</p>
					<h3 class="font-display text-2xl font-bold text-mist-100">{work.name}</h3>
					<p class="mt-1 text-sm text-mist-300">{work.description}</p>
					<div
						class="bar mt-4"
						role="progressbar"
						aria-label="Tiến độ {work.name}"
						aria-valuemin={0}
						aria-valuemax={work.goal}
						aria-valuenow={done}
					>
						<span class="fill" style:width="{progressRatio(done, work.goal) * 100}%"></span>
						{#each work.stages as s (s.decoration.slug)}
							<span class="tick" class:reached={done >= s.at} style:left="{progressRatio(s.at, work.goal) * 100}%"></span>
						{/each}
					</div>
					<p class="mt-1.5 flex justify-between text-xs text-mist-300 tabular-nums">
						<span>{formatQi(done)} / {formatQi(work.goal)} linh thạch</span>
						<span>{Math.floor(progressRatio(done, work.goal) * 100)}%</span>
					</p>
					<ul class="mt-3 grid grid-cols-3 gap-2">
						{#each work.stages as s (s.decoration.slug)}
							{@const got = mine.progress.unlocked.includes(s.decoration.slug)}
							<li class="stage" class:got>
								<CosmeticArt kind="faction_decoration" slug={s.decoration.slug} rarity={got ? 'thien' : 'pham'} size={44} />
								<span class="text-xs leading-tight">{s.decoration.name}</span>
								<span class="text-[0.7rem] text-mist-500 tabular-nums">{got ? 'Đã dựng' : `ở ${formatQi(s.at)}`}</span>
							</li>
						{/each}
					</ul>
				</div>
			{:else}
				<p class="mt-2 text-sm text-gold-300">Mọi công trình đã hoàn thành — sơn môn rực rỡ nhất cõi.</p>
			{/if}

			<div class="mt-3 flex flex-wrap items-center gap-x-4 gap-y-2 text-sm">
				<span class="text-mist-300">Hôm nay đã góp <strong class="text-mist-100 tabular-nums">{v.todayDonated.toLocaleString('vi-VN')}</strong>/{v.settings.dailyCapPerCharacter.toLocaleString('vi-VN')}</span>
				<span class="text-mist-300">Tổng của bạn <strong class="text-mist-100 tabular-nums">{mine.myTotal.toLocaleString('vi-VN')}</strong></span>
				<button class="btn btn-jade ml-auto h-10 px-4 text-sm" disabled={busy || most < v.settings.minAmount} onclick={start}>
					<Icon name="stone" size={16} /> {capLeft <= 0 ? 'Hôm nay đã góp đủ' : 'Cúng góp'}
				</button>
			</div>

			{#if mine.progress.unlocked.length}
				<h3 class="mt-5 mb-2 text-sm font-semibold text-mist-300">Sơn môn đã dựng</h3>
				<ul class="flex flex-wrap gap-2">
					{#each mine.progress.unlocked as slug (slug)}
						<li class="flex items-center gap-1.5 border border-gold-500/25 px-2 py-1 text-xs text-gold-300">
							<CosmeticArt kind="faction_decoration" slug={slug} rarity="thien" size={24} />{decoName(slug)}
						</li>
					{/each}
				</ul>
			{/if}
		</section>

		<section>
			<h2 class="panel-title">Công đức bảng</h2>
			{#if mine.donors.length}
				<ol class="mt-2 divide-y divide-gold-500/10 text-sm">
					{#each mine.donors as d, i (d.characterId)}
						<li class="flex items-center gap-3 px-2 py-1.5" class:me-row={d.characterId === game.character?.id}>
							<span class="w-6 text-right text-mist-500 tabular-nums">{i + 1}</span>
							<span class="min-w-0 flex-1 truncate text-mist-100">{d.name}</span>
							<span class="text-gold-300 tabular-nums">{d.total.toLocaleString('vi-VN')}</span>
						</li>
					{/each}
				</ol>
			{:else}
				<p class="mt-2 text-sm text-mist-500">Chưa ai cúng góp — hãy là người đầu tiên khắc tên.</p>
			{/if}
			<p class="mt-2 text-xs text-mist-500">Công đức bảng chỉ để ghi danh: không cộng công thế lực, thứ hạng, điểm duyên hay tu vi.</p>
		</section>
	{/if}

	<section>
		<h2 class="panel-title">{compact ? 'Công trình các thế lực' : 'Các thế lực khác'}</h2>
		{#if others.length}
			<ul class="mt-2 space-y-1.5 text-sm">
				{#each others as x (x.factionId)}
					{@const p = v.works.findIndex((w) => x.donatedTotal < w.start + w.goal)}
					<li class="flex items-center gap-3">
						<span class="min-w-0 flex-1 truncate text-mist-100">
							{factionOf(x.factionId)?.name ?? x.factionId}
							{#if factionOf(x.factionId)?.open === false}<span class="text-xs text-mist-500"> · đã đóng, lưu trữ</span>{/if}
						</span>
						<span class="text-xs text-mist-500">{p < 0 ? 'Đã xong mọi công trình' : v.works[p]?.name}</span>
						<span class="w-20 text-right text-gold-300 tabular-nums">{formatQi(totalOf(x.factionId))}</span>
					</li>
				{/each}
			</ul>
		{:else}
			<p class="mt-2 text-sm text-mist-500">Chưa thế lực nào khởi công.</p>
		{/if}
		{#if compact}<p class="mt-2 text-xs text-mist-500">Gia nhập một thế lực để cùng đồng môn cúng góp xây sơn môn (chỉ là trang trí).</p>{/if}
	</section>

	<GameWindow open={open} title="Cúng góp công trình" onclose={() => (open = false)}>
		{#if work}<p class="text-mist-100">Góp linh thạch xây <strong>{work.name}</strong>.</p>{/if}
		<div class="mt-4 flex flex-wrap gap-2">
			{#each presets as p (p)}
				<button class="btn btn-ghost h-10 px-3 text-sm tabular-nums" disabled={p > most} onclick={() => (amount = p)}>{p.toLocaleString('vi-VN')}</button>
			{/each}
		</div>
		<label class="mt-4 block text-sm text-mist-300">
			Số linh thạch
			<input
				type="number"
				class="input mt-1 w-full tabular-nums"
				min={v.settings.minAmount}
				max={most}
				step="10"
				bind:value={amount}
			/>
		</label>
		<dl class="mt-3 space-y-1 text-sm">
			<div class="flex justify-between"><dt class="text-mist-300">Còn góp được hôm nay</dt><dd class="tabular-nums">{capLeft.toLocaleString('vi-VN')}</dd></div>
			<div class="flex justify-between"><dt class="text-mist-300">Đang có</dt><dd class="tabular-nums">{v.balance.toLocaleString('vi-VN')} linh thạch</dd></div>
		</dl>
		<p class="mt-3 text-xs text-mist-500">Linh thạch góp vào chỉ để xây trang trí sơn môn — không đổi lại công, thứ hạng hay tu vi.</p>
		<button
			class="btn btn-jade mt-4 w-full"
			disabled={busy || !Number.isInteger(amount) || amount < v.settings.minAmount || amount > most}
			onclick={donate}
		>
			<Icon name="check" size={16} /> Góp {Number.isInteger(amount) ? amount.toLocaleString('vi-VN') : ''} linh thạch
		</button>
	</GameWindow>
{/if}

<style>
	.work {
		border: 1px solid color-mix(in oklab, var(--fc, var(--color-gold-500)) 35%, transparent);
		background:
			radial-gradient(ellipse at 80% 0%, color-mix(in oklab, var(--fc, var(--color-gold-500)) 14%, transparent), transparent 60%),
			color-mix(in oklab, var(--color-ink-950) 55%, transparent);
	}
	.bar {
		position: relative;
		height: 0.6rem;
		background: color-mix(in oklab, var(--color-ink-950) 80%, transparent);
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 25%, transparent);
	}
	.fill {
		position: absolute;
		inset: 0 auto 0 0;
		background: linear-gradient(90deg, var(--color-jade-700), var(--color-jade-300));
		transition: width 600ms var(--ease-qi);
	}
	.tick {
		position: absolute;
		top: -3px;
		bottom: -3px;
		width: 2px;
		margin-left: -1px;
		background: var(--color-mist-500);
	}
	.tick.reached {
		background: var(--color-gold-300);
	}
	.stage {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 0.2rem;
		padding: 0.5rem 0.25rem;
		text-align: center;
		color: var(--color-mist-300);
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 15%, transparent);
	}
	.stage.got {
		color: var(--color-gold-300);
		border-color: color-mix(in oklab, var(--color-gold-300) 45%, transparent);
	}
	.me-row {
		background: color-mix(in oklab, var(--color-jade-500) 10%, transparent);
	}
	.input {
		height: 2.75rem;
		padding: 0 0.75rem;
		background: var(--color-ink-950);
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 30%, transparent);
		color: var(--color-mist-100);
	}
</style>
