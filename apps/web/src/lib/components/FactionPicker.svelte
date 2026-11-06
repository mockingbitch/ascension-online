<!-- The five starting factions as cards (hover = glow + lift); clicking opens the detail with a join confirmation. -->
<script lang="ts">
	import { factionColor, factions, type Faction } from '$lib/mock/factions';
	import FactionEmblem from './FactionEmblem.svelte';
	import GameWindow from './GameWindow.svelte';
	import Icon from './Icon.svelte';

	let { current = null, onjoin }: { current?: string | null; onjoin: (f: Faction) => void } = $props();
	let open = $state<Faction | null>(null);
</script>

<ul class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 2xl:grid-cols-5">
	{#each factions as f (f.id)}
		<li>
			<button type="button" class="card w-full text-left" class:is-current={current === f.id} style:--fc={factionColor(f)} onclick={() => (open = f)}>
				<div class="art grid place-items-center py-6">
					<span class="emblem"><FactionEmblem id={f.id} size={88} /></span>
				</div>
				<div class="px-4 pb-4">
					<p class="eyebrow">{f.path === 'vu' ? 'Đạo đồ Vu · luyện thể' : 'Đạo đồ Tiên · luyện khí'}</p>
					<h3 class="mt-1 font-display text-2xl font-bold" style:color="var(--fc)">{f.name}</h3>
					<p class="mt-1 text-sm text-mist-300 italic">“{f.motto}”</p>
					<p class="more mt-2 text-sm text-mist-100">{f.description}</p>
					{#if current === f.id}<p class="mt-2 flex items-center gap-1 text-sm text-jade-300"><Icon name="check" size={14} /> Đã gia nhập</p>{/if}
				</div>
			</button>
		</li>
	{/each}
</ul>

<GameWindow open={!!open} title={open?.name ?? ''} onclose={() => (open = null)}>
	{#if open}
		{@const f = open}
		<div class="flex items-center gap-4" style:color={factionColor(f)}>
			<FactionEmblem id={f.id} size={84} />
			<div class="min-w-0">
				<p class="text-sm text-mist-300 italic">“{f.motto}”</p>
				<p class="mt-1 text-xs text-mist-500">Chủ sự: {f.leaderTitle} · {f.path === 'vu' ? 'Đạo đồ Vu (luyện thể)' : 'Đạo đồ Tiên (luyện khí)'}</p>
			</div>
		</div>
		<p class="mt-4 leading-relaxed text-mist-100">{f.description}</p>
		<h4 class="panel-title mt-5 text-base">Đặc điểm</h4>
		<ul class="mt-2 space-y-1.5 text-sm text-mist-300">
			{#each f.traits as t (t)}<li class="flex items-center gap-2"><Icon name="spark" size={14} class="text-gold-500" /> {t}</li>{/each}
		</ul>
		<p class="mt-4 text-xs text-mist-500">Thế lực không cộng sức mạnh hay tốc độ tu luyện. Lựa chọn ở bản xem trước chỉ lưu trên trình duyệt này, chưa ghi lên máy chủ.</p>
		<button class="btn btn-gold mt-4 h-12 w-full" disabled={current === f.id} onclick={() => { onjoin(f); open = null; }}>
			{current === f.id ? 'Bạn đã ở thế lực này' : `Gia nhập ${f.name}`}
		</button>
	{/if}
</GameWindow>

<style>
	.card {
		display: block;
		overflow: hidden;
		border: 1px solid color-mix(in oklab, var(--fc) 35%, transparent);
		background: linear-gradient(180deg, color-mix(in oklab, var(--color-ink-800) 90%, transparent), color-mix(in oklab, var(--color-ink-950) 94%, transparent));
		transition:
			transform 220ms var(--ease-qi),
			box-shadow 220ms,
			border-color 220ms;
	}
	.card:hover,
	.card:focus-visible,
	.card.is-current {
		transform: translateY(-4px);
		border-color: var(--fc);
		box-shadow: 0 0 34px -8px var(--fc);
	}
	.art {
		color: var(--fc);
		background:
			radial-gradient(circle at 50% 50%, color-mix(in oklab, var(--fc) 26%, transparent), transparent 65%),
			var(--grain);
		border-bottom: 1px solid color-mix(in oklab, var(--fc) 25%, transparent);
	}
	.emblem {
		display: inline-block;
		transition: transform 600ms var(--ease-qi);
	}
	.card:hover .emblem {
		transform: rotate(8deg) scale(1.06);
	}
	.more {
		display: -webkit-box;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}
	.card:hover .more {
		-webkit-line-clamp: unset;
		line-clamp: unset;
	}
</style>
