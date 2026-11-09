<!--
	Other players' news as banners (GĐ-D, decisions 2026-11-09 #8): a sliding banner (tier 3–4, top-10, digests) or the
	large full-width banner in the realm's aura colour (tier 5–6, first-to-realm, rank 1; its sound is played by the news
	store through fxBus when Cài đặt allows). Under the HUD, in the top layer (manual popover, like the Toaster, which
	moves below it via data-toast-below). It never takes the pointer except its small close button, so nothing under it
	stops working; it auto-dismisses and stays readable in the ticker and the notification list. Hidden during
	full-screen modes (the queue keeps going; old news falls back to the ticker / list).
-->
<script lang="ts">
	import { page } from '$app/state';
	import { regions } from '$lib/engine/layout/regions.svelte';
	import { btMode } from '$lib/engine/mode/breakthrough.svelte';
	import { isModePath } from '$lib/engine/scene/routes';
	import { settings } from '$lib/engine/settings.svelte';
	import { game } from '$lib/game/store.svelte';
	import { news } from '$lib/game/news.svelte';
	import Icon from './Icon.svelte';

	const b = $derived(news.banner);
	const inMode = $derived(btMode.phase !== 'idle' || isModePath(page.url.pathname));
	const realm = $derived(b?.item.realmId ? game.catalogue?.realms.get(b.item.realmId) : undefined);
	const aura = $derived(realm?.visual.aura ?? (b?.item.kind === 'rank_top10' ? 'golden' : 'jade'));
	const top = $derived(Math.round(regions.sizes.hud) + 6);
	const right = $derived(Math.round(regions.sizes.rail));
	const label = $derived.by(() => {
		const k = b?.item.kind;
		if (k === 'first_to_realm') return 'Người đầu tiên';
		if (k === 'rank_top10') return 'Thiên Bảng';
		if (k === 'digest') return 'Thiên hạ đột phá';
		return b?.item.tribulation ? 'Vượt kiếp' : 'Đột phá';
	});
	let root = $state<HTMLDivElement>();
	$effect(() => {
		void b?.item.id;
		const el = root;
		if (!el?.showPopover) return;
		try {
			if (el.matches(':popover-open')) el.hidePopover();
			if (b && !inMode) el.showPopover();
		} catch {
			/* popover unsupported: fixed + --z-toast */
		}
	});
</script>

<div bind:this={root} class="news-layer" popover="manual" style:top="{top}px" style:right="{right}px" data-fx={settings.fx}>
	{#if b && !inMode}
		{#key b.item.id}
			<div
				class="news"
				class:is-major={b.prominence === 'major'}
				style:--aura="var(--color-aura-{aura})"
				role="status"
				aria-live="polite"
				data-news-banner={b.prominence}
				data-news-kind={b.item.kind}
				data-toast-below
			>
				<span class="mark" aria-hidden="true"><Icon name={b.item.kind === 'rank_top10' ? 'rank' : 'bolt'} size={b.prominence === 'major' ? 22 : 18} /></span>
				<span class="min-w-0 flex-1">
					<span class="eyebrow block">{label}{realm ? ` · ${realm.name}` : ''}</span>
					<span class="text block">{b.item.text}</span>
				</span>
				<button type="button" class="close" onclick={() => news.dismiss()} aria-label="Đóng thông báo"><Icon name="cross" size={16} /></button>
			</div>
		{/key}
	{/if}
</div>

<style>
	.news-layer {
		position: fixed;
		left: 0;
		z-index: var(--z-toast); /* fallback when popovers are unsupported */
		inset-block-end: auto;
		width: auto;
		height: auto;
		margin: 0;
		padding: 0 0.5rem;
		border: 0;
		background: transparent;
		color: inherit;
		overflow: visible;
		pointer-events: none;
		display: flex;
		justify-content: center;
	}
	.news-layer:not(:popover-open) {
		display: flex;
	}
	.news {
		display: flex;
		align-items: center;
		gap: 0.6rem;
		width: min(30rem, 100%);
		padding: 0.45rem 0.5rem 0.45rem 0.7rem;
		border: 1px solid color-mix(in oklab, var(--aura) 55%, transparent);
		border-left-width: 3px;
		background: color-mix(in oklab, var(--color-ink-950) 90%, transparent);
		backdrop-filter: blur(6px);
		box-shadow: 0 10px 28px -14px var(--color-ink-950);
		pointer-events: none;
		animation: slide-in var(--dur-base) var(--ease-qi);
	}
	.news.is-major {
		width: 100%;
		max-width: none;
		padding: 0.65rem 0.75rem;
		border: 0;
		border-block: 1px solid color-mix(in oklab, var(--aura) 70%, var(--color-gold-300));
		background:
			linear-gradient(90deg, transparent, color-mix(in oklab, var(--aura) 22%, transparent) 20%, color-mix(in oklab, var(--aura) 22%, transparent) 80%, transparent),
			color-mix(in oklab, var(--color-ink-950) 88%, transparent);
	}
	.mark {
		display: grid;
		place-items: center;
		flex-shrink: 0;
		color: var(--aura);
	}
	.eyebrow {
		color: color-mix(in oklab, var(--aura) 70%, var(--color-gold-300));
	}
	/* two lines at most: the whole sentence fits on phones too (≤ 90 chars) */
	.text {
		font-size: 0.9rem;
		line-height: 1.3;
		color: var(--color-mist-100);
		overflow: hidden;
		display: -webkit-box;
		-webkit-box-orient: vertical;
		-webkit-line-clamp: 2;
		line-clamp: 2;
	}
	.is-major .text {
		font-family: var(--font-display);
		font-size: 1.15rem;
		font-weight: 600;
		white-space: normal;
		text-align: center;
	}
	.is-major .eyebrow {
		text-align: center;
	}
	.close {
		display: grid;
		place-items: center;
		flex-shrink: 0;
		width: var(--tap, 44px);
		height: var(--tap, 44px);
		color: var(--color-mist-300);
		pointer-events: auto;
	}
	/* motion: slide in; the large banner also sweeps a light across (full only); off → static */
	@keyframes slide-in {
		from {
			opacity: 0;
			transform: translateY(-10px);
		}
	}
	:global(:root[data-fx='full']) .is-major {
		background-size: 200% 100%, auto;
		animation:
			slide-in var(--dur-base) var(--ease-qi),
			sweep 2.4s var(--ease-in-out-sine) 1;
	}
	@keyframes sweep {
		from {
			background-position: 100% 0, 0 0;
		}
		to {
			background-position: 0 0, 0 0;
		}
	}
</style>
