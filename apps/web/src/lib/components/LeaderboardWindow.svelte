<!-- Thiên Bảng inside the game shell. Rows open the player card (report / mention live there). -->
<script lang="ts">
	import { api, ApiError } from '$lib/api/client';
	import { game } from '$lib/game/store.svelte';
	import type { LeaderboardPage } from '$lib/game/types';
	import { openWindow } from '$lib/game/windows';
	import Icon from './Icon.svelte';
	import RankMedal from './RankMedal.svelte';
	import RealmBadge from './RealmBadge.svelte';

	const PAGE = 20;
	let offset = $state(0);
	let data = $state<LeaderboardPage | null>(null);
	let error = $state<string | null>(null);

	$effect(() => {
		const o = offset;
		error = null;
		api.leaderboard(o, PAGE).then(
			(p) => (data = p),
			(e) => (error = e instanceof ApiError ? e.message : String(e))
		);
	});
	const cat = $derived(game.catalogue);
</script>

<p class="mb-3 text-sm text-mist-300">Xếp theo cảnh giới, tầng, rồi tiến độ trong tầng. Cùng thứ hạng: ai đạt trước đứng trước.</p>

{#if error}
	<p role="alert" class="text-cinnabar-300">{error}</p>
{:else if !data}
	<p class="text-mist-500">Đang mở Thiên Bảng…</p>
{:else if data.entries.length === 0}
	<p class="py-6 text-center text-mist-300">Thiên Bảng còn trống — hãy là người đầu tiên khắc tên.</p>
{:else}
	<ol class="divide-y divide-gold-500/10">
		{#each data.entries as e (e.characterId)}
			{@const realm = cat?.realmOf(e.stageId)}
			<li>
				<button
					type="button"
					class="row flex w-full items-center gap-3 px-2 py-2.5 text-left"
					class:me={e.characterId === game.character?.id}
					onclick={() => openWindow({ kind: 'profile', characterId: e.characterId, name: e.name, stageId: e.stageId })}
				>
					<RankMedal rank={e.rank} size={34} />
					<span class="min-w-0 flex-1">
						<span class="block truncate font-medium text-mist-100">{e.name}</span>
						<span class="block truncate text-xs text-mist-500">{cat?.stageLabel(e.stageId)}</span>
					</span>
					{#if realm}<RealmBadge name={realm.name} aura={realm.visual.aura} size="sm" />{/if}
				</button>
			</li>
		{/each}
	</ol>
	<div class="mt-3 flex justify-between text-sm">
		<button class="btn btn-ghost" disabled={offset === 0} onclick={() => (offset = Math.max(0, offset - PAGE))}>
			<Icon name="arrow" size={16} class="rotate-180" /> Trước
		</button>
		<span class="self-center text-mist-500 tabular-nums">{offset + 1}–{offset + data.entries.length} / {data.total}</span>
		<button class="btn btn-ghost" disabled={offset + PAGE >= data.total} onclick={() => (offset += PAGE)}>
			Sau <Icon name="arrow" size={16} />
		</button>
	</div>
{/if}

<style>
	.row:hover {
		background: color-mix(in oklab, var(--color-gold-500) 8%, transparent);
	}
	.me {
		background: linear-gradient(90deg, color-mix(in oklab, var(--color-gold-500) 14%, transparent), transparent);
	}
</style>
