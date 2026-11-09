<!--
	Look picker (decisions 2026-11-09 "Ngoại hình…", GĐ-A; "Bộ trang phục vẽ sẵn…"): dáng người, trang phục (3 painted
	outfits per body — robe colour + hair accessory come with the outfit, no free mixing) and tư thế thiền for one path,
	with a live preview (portrait + full figure). Previews and outfit cards show only art painting EXACTLY that outfit
	(the body folder for the default outfit, characters/<body>/<outfit>/… for the others), else the SVG figure in the
	outfit's colours — so every choice visibly changes the preview even before an outfit's art exists. Used by the onboarding step and by Nhân Vật › Trang phục
	("Gương Thủy Nguyệt"). It edits a DRAFT only — the server validates path, ownership and the body cooldown on save.
	`lockedBody` (cooldown) keeps every body except `currentBody` disabled, with `lockNote` explaining why.
-->
<script lang="ts">
	import { charArtFirst } from '$lib/game/charArt.svelte';
	import { cosmetics } from '$lib/game/cosmetics.svelte';
	import { avatarSlots, bodyOf, figureOf, meditateSlots, optionsFor, outfitOf, robeToken, withBody, withOutfit } from '$lib/game/looks';
	import type { AppearanceCatalog, Aura, BodySex, Look, PathId } from '$lib/game/types';
	import CultivatorFigure from './CultivatorFigure.svelte';
	import Icon from './Icon.svelte';
	import Portrait from './Portrait.svelte';

	interface Props {
		catalog: AppearanceCatalog;
		path: PathId;
		look: Look;
		aura: Aura;
		frameId?: string;
		currentBody?: string;
		lockedBody?: boolean;
		lockNote?: string;
	}
	let { catalog, path, look = $bindable(), aura, frameId, currentBody, lockedBody = false, lockNote = '' }: Props = $props();

	const opts = $derived(optionsFor(catalog, path));
	const body = $derived(bodyOf(catalog, look.bodyId));
	const figure = $derived(figureOf(catalog, look));
	// svelte-ignore state_referenced_locally
	let sex = $state<BodySex>(bodyOf(catalog, look.bodyId)?.sex ?? 'nam');
	const shownBodies = $derived(opts.bodies.filter((b) => b.sex === sex));
	const outfit = $derived(outfitOf(catalog, look));
	const exact = $derived({ defaultOutfit: body?.defaultOutfit, exact: true });
	const preview = $derived(charArtFirst(meditateSlots({ ...look, outfitId: outfit?.id }, exact)));
	const hairName = (id: string) => catalog.hairAccessories.find((h) => h.id === id)?.name ?? id;
	const uid = $props.id();
	// Portrait resolves looks through the shared store (also outside /play, e.g. onboarding)
	$effect(() => void cosmetics.load(true));

	function pickBody(id: string) {
		const next = withBody(catalog, look, id);
		if (next) look = next;
	}
	function pickOutfit(id: string) {
		look = withOutfit(catalog, look, id);
	}
	const sexes: { id: BodySex; label: string }[] = [
		{ id: 'nam', label: 'Nam' },
		{ id: 'nu', label: 'Nữ' }
	];
</script>

<div class="picker grid gap-4 md:grid-cols-[minmax(0,15rem)_minmax(0,1fr)]">
	<!-- live preview: what the scene and every portrait will show -->
	<section class="preview relative flex flex-col items-center overflow-hidden p-3 text-center" style:--aura="var(--color-aura-{aura})" aria-label="Xem trước">
		<div class="halo absolute inset-0" aria-hidden="true"></div>
		<div class="relative aspect-square w-full max-w-[13rem]" data-testid="look-preview" data-outfit={outfit?.id} data-art={preview ? 'raster' : 'svg'}>
			{#if preview}
				<img src={preview.src} srcset={preview.srcset} alt="" class="size-full object-contain" draggable="false" />
			{:else}
				<CultivatorFigure {aura} {figure} class="size-full" />
			{/if}
		</div>
		<div class="relative -mt-6"><Portrait {aura} size={64} {frameId} {look} exact /></div>
		{#if body}
			<p class="relative mt-2 font-display text-xl font-bold text-mist-100">{body.name}</p>
			{#if outfit}<p class="relative text-sm text-mist-100" data-testid="look-outfit-name">{outfit.name}</p>{/if}
			<p class="relative text-xs text-gold-300">{body.temperament}</p>
			<p class="relative mt-1 hidden text-xs leading-relaxed text-mist-300 sm:block">{body.traits}</p>
		{/if}
	</section>

	<div class="min-w-0 space-y-4">
		<section aria-labelledby="{uid}-body">
			<div class="mb-2 flex items-center justify-between gap-2">
				<h3 id="{uid}-body" class="text-sm font-semibold text-mist-300">Dáng người</h3>
				<div class="seg flex" role="radiogroup" aria-label="Giới">
					{#each sexes as x (x.id)}
						<button type="button" role="radio" aria-checked={sex === x.id} class="seg-btn press" class:is-on={sex === x.id} onclick={() => (sex = x.id)}>{x.label}</button>
					{/each}
				</div>
			</div>
			{#if lockedBody && lockNote}<p class="mb-2 flex items-center gap-1.5 text-xs text-mist-300"><Icon name="clock" size={13} /> {lockNote}</p>{/if}
			<ul class="grid grid-cols-2 gap-2 sm:grid-cols-4" role="radiogroup" aria-labelledby="{uid}-body">
				{#each shownBodies as b (b.id)}
					{@const on = b.id === look.bodyId}
					{@const locked = lockedBody && b.id !== currentBody}
					<li>
						<button
							type="button"
							role="radio"
							aria-checked={on}
							disabled={locked}
							class="body-card press flex w-full flex-col items-center gap-1 px-2 pt-2 pb-2 text-center"
							class:is-on={on}
							data-body={b.id}
							onclick={() => pickBody(b.id)}
						>
							<span class="thumb block size-16"><CultivatorFigure {aura} figure={figureOf(catalog, withBody(catalog, look, b.id))} crop="bust" still class="size-full" /></span>
							<span class="text-sm font-medium text-mist-100">{b.name}</span>
							<span class="line-clamp-1 text-[0.7rem] text-mist-500">{b.temperament}</span>
						</button>
					</li>
				{/each}
			</ul>
		</section>

		{#if body}
			<section aria-labelledby="{uid}-outfit">
				<h3 id="{uid}-outfit" class="mb-2 text-sm font-semibold text-mist-300">Trang phục</h3>
				<ul class="grid grid-cols-3 gap-2" role="radiogroup" aria-labelledby="{uid}-outfit">
					{#each body.outfits as o (o.id)}
						{@const on = o.id === outfit?.id}
						{@const ol = withOutfit(catalog, look, o.id)}
						{@const thumb = charArtFirst(avatarSlots(ol, exact))}
						<li>
							<button
								type="button"
								role="radio"
								aria-checked={on}
								class="body-card press flex h-full w-full flex-col items-center gap-1 px-1.5 pt-2 pb-2 text-center"
								class:is-on={on}
								data-outfit={o.id}
								onclick={() => pickOutfit(o.id)}
							>
								<span class="thumb block size-16">
									{#if thumb}
										<img src={thumb.src} srcset={thumb.srcset} alt="" class="size-full object-cover" draggable="false" />
									{:else}
										<CultivatorFigure {aura} figure={figureOf(catalog, ol)} crop="bust" still class="size-full" />
									{/if}
								</span>
								<span class="text-[0.8rem] leading-tight font-medium text-mist-100">{o.name}</span>
								<span class="flex items-center gap-1 text-[0.7rem] text-mist-500">
									<span class="swatch swatch-sm" style:--c={robeToken(o.robePaletteId)} aria-hidden="true"></span>
									<span class="line-clamp-1">{hairName(o.hairId)}</span>
								</span>
							</button>
						</li>
					{/each}
				</ul>
			</section>
		{/if}

		<section aria-labelledby="{uid}-pose">
			<h3 id="{uid}-pose" class="mb-2 text-sm font-semibold text-mist-300">Tư thế thiền</h3>
			<div class="flex flex-wrap gap-2" role="radiogroup" aria-labelledby="{uid}-pose">
				{#each opts.poses as o (o.id)}
					<button type="button" role="radio" aria-checked={look.poseId === o.id} class="chip press" class:is-on={look.poseId === o.id}
						onclick={() => (look = { ...look, poseId: o.id })}>{o.name}</button>
				{/each}
			</div>
		</section>
	</div>
</div>

<style>
	.preview {
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 25%, transparent);
		background: color-mix(in oklab, var(--color-ink-950) 60%, transparent);
	}
	.halo {
		background: radial-gradient(circle at 50% 38%, color-mix(in oklab, var(--aura) 26%, transparent), transparent 60%);
	}
	.seg {
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 30%, transparent);
	}
	.seg-btn {
		min-width: 3.25rem;
		min-height: var(--tap, 44px);
		padding: 0 0.75rem;
		font-size: 0.85rem;
		color: var(--color-mist-300);
	}
	.seg-btn.is-on {
		color: var(--color-ink-950);
		background: var(--color-gold-300);
		font-weight: 600;
	}
	.body-card {
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 20%, transparent);
		background: color-mix(in oklab, var(--color-ink-950) 55%, transparent);
		transition: border-color var(--dur-fast) var(--ease-qi);
	}
	.body-card.is-on {
		border-color: var(--color-gold-300);
		background: color-mix(in oklab, var(--color-gold-500) 10%, var(--color-ink-950));
	}
	.body-card:disabled {
		opacity: 0.45;
	}
	.thumb {
		border-radius: 50%;
		overflow: hidden;
		background: radial-gradient(circle at 50% 60%, color-mix(in oklab, var(--color-jade-500) 18%, var(--color-ink-800)), var(--color-ink-950));
	}
	.chip {
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
		min-height: var(--tap, 44px);
		padding: 0 0.85rem;
		font-size: 0.85rem;
		color: var(--color-mist-100);
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 25%, transparent);
		background: color-mix(in oklab, var(--color-ink-950) 55%, transparent);
	}
	.chip.is-on {
		border-color: var(--color-gold-300);
		color: var(--color-gold-300);
		font-weight: 600;
	}
	.swatch {
		width: 1rem;
		height: 1rem;
		border-radius: 50%;
		background: var(--c);
		box-shadow: 0 0 0 1px color-mix(in oklab, var(--color-mist-100) 40%, transparent);
	}
	.swatch.swatch-sm {
		width: 0.7rem;
		height: 0.7rem;
		flex: none;
	}
</style>
