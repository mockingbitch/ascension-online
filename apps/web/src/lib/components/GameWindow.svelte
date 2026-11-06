<!--
	A window over the game scene: centred panel on desktop, bottom sheet on mobile. Controlled by `open`;
	Esc / backdrop click / ✕ call onclose (the shell maps that to history.back(), so Back closes it too).
-->
<script lang="ts">
	import type { Snippet } from 'svelte';
	import Icon from './Icon.svelte';

	interface Props {
		open: boolean;
		title: string;
		onclose: () => void;
		/** wide windows (leaderboard, Tiên Duyên) */
		size?: 'md' | 'lg';
		children: Snippet;
	}
	let { open, title, onclose, size = 'md', children }: Props = $props();
	let dialog = $state<HTMLDialogElement>();

	$effect(() => {
		if (!dialog) return;
		if (open && !dialog.open) dialog.showModal();
		else if (!open && dialog.open) dialog.close();
	});
</script>

<dialog
	bind:this={dialog}
	class="window panel text-mist-100"
	class:is-lg={size === 'lg'}
	aria-label={title}
	oncancel={(e) => {
		e.preventDefault(); // Esc: let the shell pop history (keeps URL/back stack consistent)
		onclose();
	}}
	onclick={(e) => {
		if (e.target === dialog) onclose(); // backdrop
	}}
>
	{#if open}
		<header class="flex items-center gap-3 border-b border-gold-500/15 px-5 py-3">
			<h2 class="flex-1 truncate font-display text-2xl font-bold text-gold-300">{title}</h2>
			<button type="button" class="-mr-2 inline-flex size-10 items-center justify-center text-mist-300 hover:text-mist-100" onclick={onclose}>
				<Icon name="cross" label="Đóng" />
			</button>
		</header>
		<div class="window-body min-h-0 flex-1 overflow-y-auto overscroll-contain px-5 py-4">
			{@render children()}
		</div>
	{/if}
</dialog>

<style>
	.window {
		margin: auto auto 0;
		width: 100%;
		max-width: 100%;
		max-height: 88dvh;
		padding: 0;
		flex-direction: column;
		border-bottom: 0;
		animation: sheet-in 260ms var(--ease-qi);
	}
	.window[open] {
		display: flex;
	}
	.window::backdrop {
		background: color-mix(in oklab, var(--color-ink-950) 65%, transparent);
		backdrop-filter: blur(2px);
	}
	@media (min-width: 768px) {
		.window {
			margin: auto;
			width: min(36rem, calc(100vw - 2rem));
			max-height: min(80dvh, 46rem);
			border-bottom: revert;
			animation-name: window-in;
		}
		.window.is-lg {
			width: min(56rem, calc(100vw - 2rem));
		}
	}
	@keyframes sheet-in {
		from {
			transform: translateY(24px);
			opacity: 0;
		}
	}
	@keyframes window-in {
		from {
			transform: scale(0.97);
			opacity: 0;
		}
	}
</style>
