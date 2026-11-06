<script lang="ts">
	import Portrait from '$lib/components/Portrait.svelte';
	import Icon from '$lib/components/Icon.svelte';
	import RankMedal from '$lib/components/RankMedal.svelte';
	import RealmBadge from '$lib/components/RealmBadge.svelte';

	let { data } = $props();
	const cat = $derived(data.catalogue);

	const firstPage = $derived(data.page?.offset === 0);
	const podium = $derived(firstPage ? (data.page?.entries.slice(0, 3) ?? []) : []);
	const rest = $derived(firstPage ? (data.page?.entries.slice(3) ?? []) : (data.page?.entries ?? []));
	// Source order = rank order (mobile stacks 1·2·3); from `sm` the podium reads 2 · 1 · 3.
	const podiumOrder = $derived(podium);
	const smOrder = ['', 'sm:order-2', 'sm:order-1', 'sm:order-3'];
	const stageOnly = (stageId: string) => {
		const s = cat.stages.get(stageId);
		const r = cat.realmOf(stageId);
		if (!s || !r) return '';
		return s.name.startsWith(r.name) ? s.name.slice(r.name.length).trim() : s.name;
	};
</script>

<svelte:head><title>Thiên Bảng · Tiên Đạo</title></svelte:head>

<header class="mb-8 text-center">
	<p class="eyebrow">Toàn cõi Thiên Nguyên Giới</p>
	<h1 class="mt-2 text-5xl font-bold text-gold-300">Thiên Bảng</h1>
	<p class="mx-auto mt-3 max-w-xl text-sm text-mist-300">
		Xếp theo cảnh giới, tầng, rồi tiến độ trong tầng. Cùng thứ hạng: ai đạt trước đứng trước.
	</p>
</header>

{#if data.error}
	<p role="alert" class="panel p-4 text-cinnabar-300">{data.error}</p>
{:else if data.page}
	{#if podiumOrder.length}
		<ol class="mx-auto mb-8 grid max-w-4xl items-end gap-4 sm:grid-cols-3">
			{#each podiumOrder as e (e.characterId)}
				{@const realm = cat.realmOf(e.stageId)}
				{@const aura = realm?.visual.aura ?? 'none'}
				<li
					class={['panel podium flex flex-col items-center px-4 pt-6 pb-5 text-center', e.rank === 1 && 'first', smOrder[e.rank]]}
					style:--aura="var(--color-aura-{aura})"
				>
					<RankMedal rank={e.rank} size={e.rank === 1 ? 52 : 44} />
					<div class="mt-4"><Portrait {aura} size={e.rank === 1 ? 112 : 88} /></div>
					<p class="mt-4 w-full truncate font-display text-2xl font-bold text-mist-100">{e.name}</p>
					<div class="mt-2">{#if realm}<RealmBadge name={realm.name} {aura} size="sm" />{/if}</div>
					<p class="mt-2 text-sm text-mist-300">{stageOnly(e.stageId)}</p>
				</li>
			{/each}
		</ol>
	{/if}

	{#if rest.length}
		<ol class="panel mx-auto max-w-4xl divide-y divide-gold-500/10">
			{#each rest as e (e.characterId)}
				{@const realm = cat.realmOf(e.stageId)}
				<li class="flex items-center gap-4 px-4 py-3">
					<RankMedal rank={e.rank} size={36} />
					<span class="min-w-0 flex-1 truncate font-medium text-mist-100">{e.name}</span>
					<span class="hidden text-sm text-mist-300 sm:inline">{stageOnly(e.stageId)}</span>
					{#if realm}<RealmBadge name={realm.name} aura={realm.visual.aura} size="sm" />{/if}
				</li>
			{/each}
		</ol>
	{:else if !podiumOrder.length}
		<p class="panel mx-auto max-w-4xl px-4 py-8 text-center text-mist-300">Thiên Bảng còn trống — hãy là người đầu tiên khắc tên.</p>
	{/if}

	<nav class="mx-auto mt-5 flex max-w-4xl justify-between text-sm">
		{#if data.page.offset > 0}
			<a class="btn btn-ghost" href="?offset={Math.max(0, data.page.offset - data.pageSize)}"><Icon name="arrow" size={16} class="rotate-180" /> Trước</a>
		{:else}<span></span>{/if}
		{#if data.page.offset + data.pageSize < data.page.total}
			<a class="btn btn-ghost" href="?offset={data.page.offset + data.pageSize}">Sau <Icon name="arrow" size={16} /></a>
		{/if}
	</nav>
{/if}

<style>
	.podium {
		background:
			radial-gradient(ellipse at 50% 0%, color-mix(in oklab, var(--aura) 16%, transparent), transparent 60%),
			var(--grain),
			linear-gradient(180deg, color-mix(in oklab, var(--color-ink-800) 90%, transparent), color-mix(in oklab, var(--color-ink-900) 94%, transparent));
	}
	.podium.first {
		border-color: color-mix(in oklab, var(--color-gold-300) 55%, transparent);
		padding-top: 2.5rem;
		padding-bottom: 2rem;
		box-shadow: 0 0 40px -10px color-mix(in oklab, var(--color-gold-300) 35%, transparent);
	}
</style>
