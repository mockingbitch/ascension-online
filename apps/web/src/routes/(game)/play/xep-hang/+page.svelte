<!-- Xếp Hạng: Thiên Bảng (server) with a Top-3 podium; faction and contribution boards are previews. -->
<script lang="ts">
	import { api, ApiError } from '$lib/api/client';
	import Icon from '$lib/components/Icon.svelte';
	import Portrait from '$lib/components/Portrait.svelte';
	import RankMedal from '$lib/components/RankMedal.svelte';
	import RealmBadge from '$lib/components/RealmBadge.svelte';
	import ScreenFrame from '$lib/components/ScreenFrame.svelte';
	import { game } from '$lib/game/store.svelte';
	import type { LeaderboardPage } from '$lib/game/types';
	import { openWindow } from '$lib/game/windows';
	import { factionColor, factions } from '$lib/mock/factions';

	type Tab = 'personal' | 'faction';
	let tab = $state<Tab>('personal');
	const PAGE = 30;
	let offset = $state(0);
	let data = $state<LeaderboardPage | null>(null);
	let error = $state<string | null>(null);
	$effect(() => {
		const o = offset;
		api.leaderboard(o, PAGE).then(
			(p) => ((data = p), (error = null)),
			(e) => (error = e instanceof ApiError ? e.message : String(e))
		);
	});

	const cat = $derived(game.catalogue!);
	const podium = $derived(offset === 0 ? (data?.entries.slice(0, 3) ?? []) : []);
	const rest = $derived(offset === 0 ? (data?.entries.slice(3) ?? []) : (data?.entries ?? []));
	const order = ['', 'sm:order-2', 'sm:order-1', 'sm:order-3'];
	const byFaction = [...factions].sort((a, b) => b.totalCultivation - a.totalCultivation);
	const profile = (e: { characterId: string; name: string; stageId: string }) => openWindow({ kind: 'profile', characterId: e.characterId, name: e.name, stageId: e.stageId });
</script>

<svelte:head><title>Xếp Hạng · Tiên Đạo</title></svelte:head>

<ScreenFrame title="Thiên Bảng" subtitle="Toàn cõi Thiên Nguyên Giới" icon="rank" status={tab === 'faction' ? 'preview' : 'live'}>
	<div class="mb-4 flex gap-1" role="tablist">
		<button role="tab" aria-selected={tab === 'personal'} class="tab" class:is-on={tab === 'personal'} onclick={() => (tab = 'personal')}>Tu sĩ</button>
		<button role="tab" aria-selected={tab === 'faction'} class="tab" class:is-on={tab === 'faction'} onclick={() => (tab = 'faction')}>Thế lực</button>
	</div>

	{#if tab === 'personal'}
		{#if error}
			<p role="alert" class="text-cinnabar-300">{error}</p>
		{:else if !data}
			<p class="text-mist-500">Đang mở Thiên Bảng…</p>
		{:else}
			{#if podium.length}
				<ol class="mx-auto mb-5 grid max-w-3xl items-end gap-3 sm:grid-cols-3">
					{#each podium as e (e.characterId)}
						{@const r = cat.realmOf(e.stageId)}
						<li class={['podium', e.rank === 1 && 'first', order[e.rank]]} style:--aura="var(--color-aura-{r?.visual.aura ?? 'none'})">
							<button type="button" class="flex w-full flex-col items-center px-3 pt-5 pb-4 text-center" onclick={() => profile(e)}>
								<RankMedal rank={e.rank} size={e.rank === 1 ? 48 : 40} />
								<div class="mt-3"><Portrait aura={r?.visual.aura ?? 'none'} size={e.rank === 1 ? 96 : 76} /></div>
								<p class="mt-3 w-full truncate font-display text-xl font-bold text-mist-100">{e.name}</p>
								{#if r}<div class="mt-1"><RealmBadge name={r.name} aura={r.visual.aura} size="sm" /></div>{/if}
							</button>
						</li>
					{/each}
				</ol>
			{/if}
			<ol class="divide-y divide-gold-500/10">
				{#each rest as e (e.characterId)}
					{@const r = cat.realmOf(e.stageId)}
					<li>
						<button type="button" class="row flex w-full items-center gap-3 px-2 py-2.5 text-left" class:me={e.characterId === game.character?.id} onclick={() => profile(e)}>
							<RankMedal rank={e.rank} size={34} />
							<span class="min-w-0 flex-1">
								<span class="block truncate font-medium text-mist-100">{e.name}</span>
								<span class="block truncate text-xs text-mist-500">{cat.stageLabel(e.stageId)}</span>
							</span>
							{#if r}<RealmBadge name={r.name} aura={r.visual.aura} size="sm" />{/if}
						</button>
					</li>
				{/each}
			</ol>
			<div class="mt-3 flex items-center justify-between text-sm">
				<button class="btn btn-ghost" disabled={offset === 0} onclick={() => (offset = Math.max(0, offset - PAGE))}><Icon name="arrow" size={16} class="rotate-180" /> Trước</button>
				<span class="text-mist-500 tabular-nums">Hạng của bạn: <strong class="text-gold-300">{game.myRank ?? '—'}</strong> · {data.total} tu sĩ</span>
				<button class="btn btn-ghost" disabled={offset + PAGE >= data.total} onclick={() => (offset += PAGE)}>Sau <Icon name="arrow" size={16} /></button>
			</div>
		{/if}
	{:else}
		<ol class="space-y-2">
			{#each byFaction as f, i (f.id)}
				<li class="frow flex items-center gap-3 px-3 py-3" style:--fc={factionColor(f)}>
					<RankMedal rank={i + 1} size={34} />
					<span class="min-w-0 flex-1">
						<span class="block truncate font-display text-xl font-bold" style:color="var(--fc)">{f.name}</span>
						<span class="block truncate text-xs text-mist-500">{f.members.toLocaleString('vi-VN')} đệ tử (mẫu)</span>
					</span>
					<span class="text-right text-sm tabular-nums text-mist-100">{f.totalCultivation.toLocaleString('vi-VN')}<span class="block text-xs text-mist-500">tổng tu vi (mẫu)</span></span>
				</li>
			{/each}
		</ol>
	{/if}
</ScreenFrame>

<style>
	.tab {
		height: 2.25rem;
		padding: 0 1rem;
		font-size: 0.875rem;
		color: var(--color-mist-300);
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 20%, transparent);
	}
	.tab.is-on {
		color: var(--color-ink-950);
		font-weight: 600;
		background: linear-gradient(180deg, var(--color-gold-300), var(--color-gold-500));
	}
	.podium {
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 30%, transparent);
		background:
			radial-gradient(ellipse at 50% 0%, color-mix(in oklab, var(--aura) 18%, transparent), transparent 60%),
			color-mix(in oklab, var(--color-ink-950) 45%, transparent);
	}
	.podium.first {
		border-color: color-mix(in oklab, var(--color-gold-300) 60%, transparent);
		box-shadow: 0 0 40px -12px color-mix(in oklab, var(--color-gold-300) 45%, transparent);
	}
	.row:hover {
		background: color-mix(in oklab, var(--color-gold-500) 8%, transparent);
	}
	.me {
		background: linear-gradient(90deg, color-mix(in oklab, var(--color-gold-500) 14%, transparent), transparent);
	}
	.frow {
		border: 1px solid color-mix(in oklab, var(--fc) 35%, transparent);
		background: linear-gradient(90deg, color-mix(in oklab, var(--fc) 12%, transparent), transparent 60%);
	}
</style>
