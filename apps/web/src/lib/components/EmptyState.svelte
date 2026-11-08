<!--
	Empty state (spec §3.4): a small ink-wash illustration (inline SVG, token colours), one line saying what is empty and
	a hint of the action that fills it, optionally as a button.
-->
<script lang="ts">
	import Icon, { type IconName } from './Icon.svelte';

	interface Props {
		icon: IconName;
		title: string;
		hint?: string;
		action?: { label: string; icon?: IconName; onclick: () => void };
	}
	let { icon, title, hint, action }: Props = $props();
</script>

<div class="empty flex flex-col items-center px-4 py-8 text-center" data-empty>
	<svg viewBox="0 0 160 96" class="art" aria-hidden="true">
		<!-- distant ink mountains, mist band, moon -->
		<circle cx="122" cy="24" r="10" class="moon" />
		<path d="M0 78 28 46l16 16 22-30 26 34 14-14 22 26 32-22v40H0Z" class="far" />
		<path d="M0 86 34 64l20 12 30-20 30 22 18-10 28 14v14H0Z" class="near" />
		<path d="M10 70h50M96 60h44M30 82h36" class="mist" />
	</svg>
	<div class="medal -mt-9 grid size-14 place-items-center rounded-full"><Icon name={icon} size={26} /></div>
	<p class="mt-3 font-display text-xl font-bold text-mist-100">{title}</p>
	{#if hint}<p class="mt-1 max-w-xs text-sm text-mist-300">{hint}</p>{/if}
	{#if action}
		<button type="button" class="btn btn-jade mt-4 h-10 px-4 text-sm" onclick={action.onclick}>
			{#if action.icon}<Icon name={action.icon} size={16} />{/if}{action.label}
		</button>
	{/if}
</div>

<style>
	.art {
		width: min(220px, 70%);
		height: auto;
	}
	.moon {
		fill: color-mix(in oklab, var(--color-gold-300) 35%, transparent);
	}
	.far {
		fill: color-mix(in oklab, var(--color-jade-700) 45%, transparent);
	}
	.near {
		fill: color-mix(in oklab, var(--color-ink-700) 90%, transparent);
	}
	.mist {
		stroke: color-mix(in oklab, var(--color-mist-300) 30%, transparent);
		stroke-width: 3;
		stroke-linecap: round;
	}
	.medal {
		position: relative;
		color: var(--color-gold-300);
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 55%, transparent);
		background: radial-gradient(circle at 50% 35%, var(--color-ink-700), var(--color-ink-950));
	}
</style>
