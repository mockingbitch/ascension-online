<!-- "Thêm" (phones): every scene and window the bottom row has no room for, as an icon grid with badges. -->
<script lang="ts">
	import Icon from '$lib/components/Icon.svelte';
	import { badgeOf } from '$lib/game/badges';
	import { extraItems, sceneItems, windowItems, type BarItem } from '$lib/game/nav';
	import { updateWindow } from '$lib/game/windows';
	import type { WindowState } from '$lib/game/windowStack';

	const groups: { title: string; items: BarItem[] }[] = [
		{ title: 'Địa danh', items: sceneItems },
		{ title: 'Đạo đồ', items: windowItems },
		{ title: 'Khác', items: extraItems }
	];
	// The menu is a launcher: the chosen window takes its place (Back returns to the scene, not to the menu).
	const open = (it: BarItem) => it.window && updateWindow({ kind: it.window } as WindowState);
</script>

{#each groups as g (g.title)}
	<h3 class="eyebrow mt-1 mb-2">{g.title}</h3>
	<ul class="stagger mb-4 grid grid-cols-4 gap-2">
		{#each g.items as it (it.id)}
			{@const b = badgeOf(it.id)}
			<li>
				{#if it.href}
					<a href={it.href} class="tile press" data-menu={it.id}>
						<Icon name={it.icon} size={24} />
						<span>{it.short}</span>
					</a>
				{:else}
					<button type="button" class="tile press" data-menu={it.id} onclick={() => open(it)}>
						<Icon name={it.icon} size={24} />
						<span>{it.short}</span>
						{#if b}<span class="badge" aria-label={b.label}>{b.text}</span>{/if}
					</button>
				{/if}
			</li>
		{/each}
	</ul>
{/each}

<style>
	.tile {
		position: relative;
		display: flex;
		width: 100%;
		min-height: 4.5rem;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 0.3rem;
		font-size: 0.72rem;
		font-weight: 600;
		color: var(--color-mist-100);
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 20%, transparent);
		background: color-mix(in oklab, var(--color-ink-950) 45%, transparent);
		border-radius: 3px;
	}
	.tile :global(svg) {
		color: var(--color-gold-300);
	}
	.badge {
		position: absolute;
		top: 4px;
		right: 4px;
		min-width: 18px;
		height: 18px;
		padding: 0 4px;
		border-radius: 9999px;
		font-size: 0.68rem;
		font-weight: 700;
		line-height: 18px;
		color: var(--color-mist-100);
		background: var(--color-cinnabar-500);
	}
</style>
