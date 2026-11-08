<!-- Toast stack at the top-centre of the game screen, above modes. It starts below the HUD, or below the lowest
     visible element a page marks `data-toast-below` (scene UI at the top centre, e.g. Động Phủ's realm plaque), so a
     toast never covers that UI. UI polish U1: a manual popover (top layer), re-raised whenever a toast appears, so a
     toast shows above an open window too (top-layer order = opening order). -->
<script lang="ts">
	import { regions } from '$lib/engine/layout/regions.svelte';
	import { dismiss, toasts } from '$lib/game/toasts.svelte';
	import Icon from './Icon.svelte';

	// below the HUD region (--hud-h, measured by the shell; outside /play the HUD default)
	const below = $derived(Math.round(regions.sizes.hud) + 8);
	let top = $state(64);
	// re-measured whenever the first toast of a burst appears (pages, scenes and windows change between toasts)
	const any = $derived(toasts.items.length > 0);
	$effect(() => {
		if (!any) return;
		let y = below;
		for (const el of document.querySelectorAll('[data-toast-below]')) {
			if (el.checkVisibility && !el.checkVisibility({ visibilityProperty: true })) continue;
			const r = el.getBoundingClientRect();
			if (r.height > 0 && r.top < innerHeight / 2) y = Math.max(y, r.bottom + 8);
		}
		top = Math.round(y);
	});
	let root = $state<HTMLDivElement>();
	// top layer while toasts are up: re-show on every new toast so it stacks above any window opened since
	const newest = $derived(toasts.items.at(-1)?.id);
	$effect(() => {
		void newest;
		const el = root;
		if (!el?.showPopover) return;
		try {
			if (el.matches(':popover-open')) el.hidePopover();
			if (any) el.showPopover();
		} catch {
			/* popover unsupported: fixed + --z-toast */
		}
	});
</script>

<div bind:this={root} class="toaster pointer-events-none fixed inset-x-0 flex flex-col items-center gap-2 px-3" popover="manual" style:top="{top}px" role="status" aria-live="polite" data-toaster>
	{#each toasts.items as t (t.id)}
		<button
			type="button"
			class="toast pointer-events-auto flex max-w-md items-start gap-2 px-4 py-2.5 text-left text-sm"
			class:tone-ok={t.tone === 'ok'}
			class:tone-warn={t.tone === 'warn'}
			class:tone-err={t.tone === 'err'}
			onclick={() => dismiss(t.id)}
		>
			<Icon name={t.tone === 'ok' ? 'spark' : t.tone === 'warn' ? 'clock' : 'cross'} size={18} class="mt-px shrink-0" />
			<span class="text-mist-100">{t.text}</span>
		</button>
	{/each}
</div>

<style>
	.toaster {
		z-index: var(--z-toast); /* above the ModeLayer (fallback when popovers are unsupported) */
		/* undo the UA popover box: a full-width strip under the HUD */
		inset: auto 0 auto 0;
		width: auto;
		height: auto;
		margin: 0;
		border: 0;
		background: transparent;
		color: inherit;
		overflow: visible;
	}
	.toaster:not(:popover-open) {
		display: flex;
	}
	.toast {
		border: 1px solid color-mix(in oklab, currentColor 45%, transparent);
		border-left-width: 3px;
		background: color-mix(in oklab, var(--color-ink-950) 88%, transparent);
		backdrop-filter: blur(6px);
		box-shadow: 0 10px 30px -12px var(--color-ink-950);
		animation: toast-in 260ms var(--ease-qi);
	}
	.tone-ok {
		color: var(--color-jade-300);
	}
	.tone-warn {
		color: var(--color-gold-300);
	}
	.tone-err {
		color: var(--color-cinnabar-300);
	}
	@keyframes toast-in {
		from {
			opacity: 0;
			transform: translateY(-8px);
		}
	}
</style>
