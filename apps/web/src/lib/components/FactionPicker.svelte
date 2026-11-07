<!-- Faction cards from GET /factions (hover = glow + lift); clicking opens the detail with a join/switch confirmation.
     Joining a faction that doesn't allow the character's path converts it (same tier and tầng, tu vi kept). -->
<script lang="ts">
	import { factionColor } from '$lib/game/faction.svelte';
	import { loreOf } from '$lib/game/factionLore';
	import type { FactionView, PathId } from '$lib/game/types';
	import FactionEmblem from './FactionEmblem.svelte';
	import GameWindow from './GameWindow.svelte';
	import Icon from './Icon.svelte';

	interface Props {
		factions: FactionView[];
		current?: string | null;
		/** the character's current path (derived from its stage) */
		path: PathId;
		/** display name of a path (from the realm catalogue) */
		pathName: (p: PathId) => string;
		busy?: boolean;
		/** label verb: 'Gia nhập' or 'Chuyển sang' */
		verb?: string;
		/** extra note shown above the confirm button (e.g. the switch cooldown) */
		note?: string;
		/** onboarding: no character yet, so the path is chosen by the faction (no "conversion" wording) */
		newcomer?: boolean;
		onjoin: (f: FactionView) => void;
	}
	let { factions, current = null, path, pathName, busy = false, verb = 'Gia nhập', note = '', newcomer = false, onjoin }: Props = $props();
	let open = $state<FactionView | null>(null);

	const pathLabel = (f: FactionView) => f.allowedPaths.map((p) => `Đạo đồ ${pathName(p)}`).join(' · ');
	const converts = (f: FactionView) => !f.allowedPaths.includes(path);
</script>

<ul class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 2xl:grid-cols-5">
	{#each factions as f (f.id)}
		{@const lore = loreOf(f.id)}
		<li>
			<button type="button" class="card w-full text-left" class:is-current={current === f.id} style:--fc={factionColor(f)} onclick={() => (open = f)}>
				<div class="art grid place-items-center py-6">
					<span class="emblem"><FactionEmblem id={f.id} size={88} /></span>
				</div>
				<div class="px-4 pb-4">
					<p class="eyebrow">{pathLabel(f)} · {f.members.toLocaleString('vi-VN')} đệ tử</p>
					<h3 class="mt-1 font-display text-2xl font-bold" style:color="var(--fc)">{f.name}</h3>
					{#if f.motto}<p class="mt-1 text-sm text-mist-300 italic">“{f.motto}”</p>{/if}
					<p class="more mt-2 text-sm text-mist-100">{lore.description || f.pitch}</p>
					{#if current === f.id}<p class="mt-2 flex items-center gap-1 text-sm text-jade-300"><Icon name="check" size={14} /> Đã gia nhập</p>{/if}
				</div>
			</button>
		</li>
	{/each}
</ul>

<GameWindow open={!!open} title={open?.name ?? ''} onclose={() => (open = null)}>
	{#if open}
		{@const f = open}
		{@const lore = loreOf(f.id)}
		<div class="flex items-center gap-4" style:color={factionColor(f)}>
			<FactionEmblem id={f.id} size={84} />
			<div class="min-w-0">
				{#if f.motto}<p class="text-sm text-mist-300 italic">“{f.motto}”</p>{/if}
				<p class="mt-1 text-xs text-mist-500">{lore.leaderTitle ? `Chủ sự: ${lore.leaderTitle} · ` : ''}{pathLabel(f)}</p>
			</div>
		</div>
		<p class="mt-4 leading-relaxed text-mist-100">{lore.description || f.pitch}</p>
		{#if lore.traits.length}
			<h4 class="panel-title mt-5 text-base">Đặc điểm</h4>
			<ul class="mt-2 space-y-1.5 text-sm text-mist-300">
				{#each lore.traits as t (t)}<li class="flex items-center gap-2"><Icon name="spark" size={14} class="text-gold-500" /> {t}</li>{/each}
			</ul>
		{/if}
		{#if newcomer}
			<p class="mt-4 flex gap-2 text-sm text-gold-300">
				<Icon name="bolt" size={16} class="mt-0.5 shrink-0" />
				{f.allowedPaths.length > 1
					? `Thế lực này nhận nhiều con đường (${f.allowedPaths.map(pathName).join(', ')}), bạn chọn ở bước sau.`
					: `Bạn sẽ bắt đầu trên con đường ${pathName(f.allowedPaths[0] ?? path)}, từ tầng đầu tiên.`}
			</p>
		{:else if converts(f) && current !== f.id}
			<p class="mt-4 flex gap-2 text-sm text-gold-300">
				<Icon name="bolt" size={16} class="mt-0.5 shrink-0" />
				Gia nhập sẽ chuyển đạo đồ của bạn sang {pathName(f.allowedPaths[0] ?? path)} — giữ nguyên cảnh giới, tầng và tu vi.
			</p>
		{/if}
		<p class="mt-4 text-xs text-mist-500">Thế lực không cộng sức mạnh hay tốc độ tu luyện.{note ? ` ${note}` : ''}</p>
		<button class="btn btn-gold mt-4 h-12 w-full" disabled={busy || current === f.id || !f.open} onclick={() => { onjoin(f); open = null; }}>
			{current === f.id ? 'Bạn đã ở thế lực này' : !f.open ? 'Thế lực chưa mở cửa' : `${verb} ${f.name}`}
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
