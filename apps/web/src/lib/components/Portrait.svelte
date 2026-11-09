<!-- Round character portrait: bust crop of the cultivator inside a double gold ring, tinted by realm aura.
     `look` = the character's chosen look (appearances.yaml, decisions 2026-11-09): the outfit's raster
     `characters/<body_id>/<outfit_id>/avatar`, else the body's (default outfit) `characters/<body_id>/avatar` (lazy;
     robe recoloured through an optional `avatar-robe` mask), else the parameterised SVG figure — never another body's
     art. `exact` (look picker): only the art of exactly this outfit, else the SVG in the outfit's colours. Without a look (older data) the shared `characters/avatar`
     slot or the default SVG is used.
     `frameId` = equipped avatar_frame cosmetic (looks only): raster `cosmetics/<slug>` overlay, CSS ring otherwise. -->
<script lang="ts">
	import { art } from '$lib/game/art';
	import { charArt, charArtFirstSlot } from '$lib/game/charArt.svelte';
	import { cosmetics, frameStyle, slugOf } from '$lib/game/cosmetics.svelte';
	import { avatarSlots, bodyOf, figureOf, isBodyLevel, maskSlot, outfitOf, robeToken } from '$lib/game/looks';
	import type { Aura, Look } from '$lib/game/types';
	import CultivatorFigure from './CultivatorFigure.svelte';

	const shared = art('characters/avatar'); // raster slot (docs/design/art-prompts.md §4), SVG otherwise

	interface Props {
		aura: Aura;
		size?: number; // px
		frameId?: string;
		look?: Look;
		/** effect tier of the character's realm (realms.yaml visual.fx_tier): a stronger static glow per tier —
		 *  from the realm only, never from anything bought (decisions 2026-11-09 #2) */
		fxTier?: number;
		/** look picker: only the art painting exactly this outfit (else the SVG in its colours) */
		exact?: boolean;
	}
	let { aura, size = 80, frameId, look, fxTier = 1, exact = false }: Props = $props();
	const frameArt = $derived(frameId ? art(`cosmetics/${slugOf(frameId)}`) : undefined);
	const figure = $derived(figureOf(cosmetics.looks, look));
	// with a look, only that body's art (the SVG while the catalogue/art loads) — never the shared art of another figure
	const outfit = $derived(outfitOf(cosmetics.looks, look));
	const hit = $derived(
		look ? charArtFirstSlot(avatarSlots({ ...look, outfitId: outfit?.id ?? look.outfitId }, { defaultOutfit: bodyOf(cosmetics.looks, look.bodyId)?.defaultOutfit, exact })) : undefined
	);
	const own = $derived(hit?.art);
	// the body-level painting can be recoloured to the outfit's robe (also when it stands in for an alternate outfit)
	const mask = $derived(look && hit && isBodyLevel(look.bodyId, hit.slot) ? charArt(maskSlot(look.bodyId, 'avatar')) : undefined);
	const avatar = $derived(look ? own : shared);
</script>

<span class="relative inline-block shrink-0" style:width="{size}px" style:height="{size}px" data-body={look?.bodyId}>
	<span
		class="portrait relative block size-full overflow-hidden rounded-full"
		class:framed={!!frameId}
		style:--aura="var(--color-aura-{aura})"
		style:--tier={Math.max(1, Math.min(6, fxTier))}
		data-fx-tier={fxTier}
	>
		{#if avatar}
			<img src={avatar.src} srcset={avatar.srcset} alt="" class="size-full object-cover" draggable="false" />
			{#if mask && look}
				<span class="robe-tint" style:--mask="url({mask.src})" style:--robe={robeToken(outfit?.robePaletteId ?? look.robePaletteId)} aria-hidden="true"></span>
			{/if}
		{:else}
			<CultivatorFigure {aura} {figure} crop="bust" still={size < 64} class="size-full" />
		{/if}
	</span>
	{#if frameId}
		{#if frameArt}
			<img src={frameArt.src} srcset={frameArt.srcset} alt="" class="frame-art" draggable="false" />
		{:else}
			<span class="cos-frame frame-{frameStyle(frameId)}" aria-hidden="true"></span>
		{/if}
	{/if}
</span>

<style>
	.portrait {
		background: radial-gradient(circle at 50% 60%, color-mix(in oklab, var(--aura) 35%, var(--color-ink-800)), var(--color-ink-950));
		box-shadow:
			0 0 0 1px var(--color-gold-500),
			0 0 0 4px var(--color-ink-950),
			0 0 0 5px color-mix(in oklab, var(--color-gold-500) 45%, transparent),
			0 0 calc(12px + var(--tier, 1) * 4px) color-mix(in oklab, var(--aura) calc(30% + var(--tier, 1) * 6%), transparent);
	}
	.portrait.framed {
		box-shadow: 0 0 calc(12px + var(--tier, 1) * 4px) color-mix(in oklab, var(--aura) calc(30% + var(--tier, 1) * 6%), transparent);
	}
	/* robe recolour: the grey mask marks the robe; the token colour is blended over it (hue/saturation only) */
	.robe-tint {
		position: absolute;
		inset: 0;
		background: var(--robe);
		mix-blend-mode: color;
		mask: var(--mask) center / cover no-repeat;
		pointer-events: none;
	}
	.frame-art {
		position: absolute;
		inset: -14%;
		width: 128%;
		height: 128%;
		max-width: none;
		pointer-events: none;
	}
</style>
