<script lang="ts">
	import Icon from '$lib/components/Icon.svelte';
	import RealmBadge from '$lib/components/RealmBadge.svelte';
	import { formatPoints } from '$lib/game/format';

	let { data } = $props();
	// Default path (tiên) ladder; Vu shares the same tiers. Reserved tiers (Thánh Nhân) are shown, not playable.
	const ladder = $derived(
		data.catalogue ? data.catalogue.realms.filter((r) => r.path === (data.catalogue?.paths.find((p) => p.default)?.id ?? 'tien')) : []
	);
</script>

<svelte:head><title>Tiên Đạo Online</title></svelte:head>

<section class="py-10 text-center sm:py-16">
	<p class="eyebrow">Thiên Nguyên Giới</p>
	<h1 class="mt-3 text-6xl font-bold text-gold-300 sm:text-8xl">Tiên Đạo</h1>
	<p class="mx-auto mt-4 max-w-lg text-lg text-mist-300">Bế quan tu luyện, vượt bình cảnh, tranh đoạt thiên hạ đệ nhất.</p>
	<a href="/cultivate" class="btn btn-gold mt-8 h-12 px-8 text-base"><Icon name="lotus" size={18} /> Bước vào tu luyện</a>
</section>

{#if data.error}
	<p role="alert" class="panel mx-auto max-w-3xl p-4 text-cinnabar-300">Không kết nối được máy chủ: {data.error}</p>
{:else if data.catalogue}
	<section class="panel mx-auto max-w-3xl p-5 sm:p-6" aria-labelledby="path-title">
		<h2 id="path-title" class="panel-title">Con đường cảnh giới</h2>
		<ol class="realm-path mt-4">
			{#each ladder as realm, i (realm.id)}
				<li class="relative flex items-center gap-4 py-2.5 pl-10">
					<span class="node absolute left-3 size-3 rotate-45 border" style:border-color="var(--color-aura-{realm.visual.aura})" aria-hidden="true"></span>
					<span class="w-6 text-sm text-mist-500 tabular-nums">{i + 1}</span>
					<span class="min-w-0 flex-1"><RealmBadge name={realm.name} aura={realm.visual.aura} size="sm" /></span>
					<span class="text-right text-sm text-mist-300">
						{#if realm.reserved}
							<span class="text-mist-500">chưa ai chạm tới</span>
						{:else}
							{realm.stages.length} tầng · từ <span class="text-mist-100 tabular-nums">{formatPoints(realm.stages[0]?.cumulativeBeforeMilli ?? 0)}</span> tu vi
							{#if realm.stages[realm.stages.length - 1]?.tribulation}<span class="hidden text-cinnabar-300 sm:inline">· lôi kiếp</span>{/if}
						{/if}
					</span>
				</li>
			{/each}
		</ol>
	</section>
{/if}

<style>
	.realm-path {
		position: relative;
	}
	.realm-path::before {
		content: '';
		position: absolute;
		left: calc(0.75rem + 5px);
		top: 1rem;
		bottom: 1rem;
		width: 1px;
		background: linear-gradient(180deg, transparent, color-mix(in oklab, var(--color-gold-500) 45%, transparent) 10%, color-mix(in oklab, var(--color-gold-500) 45%, transparent) 90%, transparent);
	}
	.node {
		background: var(--color-ink-950);
	}
</style>
