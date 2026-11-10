<!--
	A painted image that cross-fades when it changes (look picker / Trang phục previews): the previous painting stays
	until the next one is decoded, then the new one fades in over it — no flash of the SVG fallback while a lazy image
	loads. The fallback (children) shows only when there is no painting at all (none shipped, or the very first load).
	Fade length follows Cài đặt › Hiệu ứng (off / reduced motion = instant swap). Display only.
-->
<script lang="ts">
	import type { Snippet } from 'svelte';
	import { untrack } from 'svelte';
	import { fade } from 'svelte/transition';
	import { settings } from '$lib/engine/settings.svelte';
	import type { Art } from '$lib/game/art';

	interface Props {
		art: Art | undefined;
		/** the slot chain is still resolving: keep what is shown */
		pending?: boolean;
		fit?: 'contain' | 'cover';
		class?: string;
		children?: Snippet;
	}
	let { art, pending = false, fit = 'contain', class: cls = '', children }: Props = $props();
	let layers = $state<{ k: number; art: Art }[]>([]);
	let seq = 0;
	const ms = $derived(settings.fx === 'off' ? 0 : settings.fx === 'lite' ? 160 : 280);

	$effect(() => {
		const a = art;
		const wait = pending;
		if (!a) {
			if (!wait && untrack(() => layers.length)) layers = [];
			return;
		}
		const top = untrack(() => layers.at(-1));
		if (top?.art.src === a.src) return;
		let live = true;
		const im = new Image();
		if (a.srcset) im.srcset = a.srcset;
		im.src = a.src;
		const show = () => {
			if (!live) return;
			const k = ++seq;
			layers = [...layers.slice(-1), { k, art: a }];
			setTimeout(() => {
				if (layers.at(-1)?.k === k) layers = layers.slice(-1);
			}, ms + 40);
		};
		(im.decode ? im.decode() : Promise.resolve()).then(show, show);
		return () => {
			live = false;
		};
	});
</script>

<span class="artfade {cls}" data-art={layers.length ? 'raster' : 'svg'}>
	{#if layers.length}
		{#each layers as l (l.k)}
			<img in:fade={{ duration: l.k === 1 ? 0 : ms }} src={l.art.src} srcset={l.art.srcset} alt="" class="layer" class:cover={fit === 'cover'} draggable="false" />
		{/each}
	{:else}
		{@render children?.()}
	{/if}
</span>

<style>
	.artfade {
		position: relative;
		display: block;
	}
	.layer {
		position: absolute;
		inset: 0;
		width: 100%;
		height: 100%;
		object-fit: contain;
	}
	.layer.cover {
		object-fit: cover;
	}
</style>
