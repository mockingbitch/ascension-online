<!--
	A world-map place plaque (wood/jade sign with icon, name, subtitle). Locked places ("soon") show a lock and a
	toast instead of navigating. Used by the world scene (anchored over the map) and the mobile place list.
-->
<script lang="ts" module>
	import { goto } from '$app/navigation';
	import { screens, type ScreenId } from '$lib/game/nav';
	import { toast } from '$lib/game/toasts.svelte';

	/** Enter a map place: navigate (the scene transition runs from the shell) or explain the seal. */
	export function enterPlace(id: ScreenId) {
		const s = screens[id];
		if (s.status === 'soon') toast('warn', `${s.label}: cấm chế chưa giải — sẽ khai mở theo Thiên Đạo Sử.`);
		else void goto(s.href);
	}
</script>

<script lang="ts">
	import Icon from './Icon.svelte';

	interface Props {
		screen: ScreenId;
		title: string;
		subtitle: string;
		/** mobile list: full width, smaller title */
		compact?: boolean;
	}
	let { screen, title, subtitle, compact = false }: Props = $props();
	const s = $derived(screens[screen]);
</script>

<button type="button" class="plaque" class:w-full={compact} class:is-soon={s.status === 'soon'} onclick={() => enterPlace(screen)}>
	<span class="ring"><Icon name={s.status === 'soon' ? 'lock' : s.icon} size={18} /></span>
	<span class="min-w-0 text-left">
		<span class={['block font-display leading-tight font-bold', compact ? 'truncate text-base' : 'text-lg']}>{title}</span>
		<span class={['block text-xs text-mist-300', compact && 'truncate']}>{subtitle}</span>
	</span>
	{#if s.status === 'soon'}<span class="sr-only">(chưa khai mở)</span>{/if}
</button>

<style>
	.plaque {
		display: flex;
		align-items: center;
		gap: 0.6rem;
		min-height: 2.75rem;
		padding: 0.4rem 0.9rem 0.4rem 0.4rem;
		color: var(--color-gold-300);
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 55%, transparent);
		background: linear-gradient(180deg, color-mix(in oklab, var(--color-ink-800) 92%, transparent), color-mix(in oklab, var(--color-ink-950) 92%, transparent));
		box-shadow: 0 8px 24px -10px var(--color-ink-950);
		border-radius: 2px;
		pointer-events: auto;
		transition: transform var(--dur-fast) var(--ease-qi);
	}
	.plaque:hover,
	.plaque:focus-visible {
		transform: translateY(-2px);
		border-color: var(--color-gold-300);
	}
	.plaque.is-soon {
		color: var(--color-mist-500);
		border-color: color-mix(in oklab, var(--color-mist-500) 40%, transparent);
	}
	.ring {
		display: grid;
		width: 2.25rem;
		height: 2.25rem;
		flex-shrink: 0;
		place-items: center;
		border-radius: 9999px;
		border: 1px solid currentColor;
		background: color-mix(in oklab, var(--color-ink-950) 70%, transparent);
	}
</style>
