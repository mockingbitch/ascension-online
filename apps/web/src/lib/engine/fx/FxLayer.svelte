<!--
	FxLayer (spec §3.1): one full-screen canvas for particles (pooled ParticleSystem on the shared loop, DPR ≤ 2) plus
	the DOM overlay (floats, reward flights, rings, announcement scroll — FxOverlay, a lazy chunk). Never takes the
	pointer except the announcement scroll's skip. It is a manual popover so it can sit in the top layer: whenever an
	effect starts it re-raises itself above an open modal window (a reward claimed inside Nhiệm vụ flies over it).
-->
<script lang="ts">
	import type { Component } from 'svelte';
	import { settings } from '../settings.svelte';
	import { tokenResolver } from './colors';
	import { fxLayer } from './layer';
	import { fxOverlay } from './overlay.svelte';

	let canvas = $state<HTMLCanvasElement>();
	let root = $state<HTMLDivElement>();
	let Overlay = $state.raw<Component | null>(null);

	$effect(() => fxLayer.setQuality(settings.fx));
	$effect(() => {
		const c = canvas;
		if (!c) return;
		const colors = tokenResolver(); // design tokens → canvas colours, resolved once per colour
		const fit = () => fxLayer.particles.resize(innerWidth, innerHeight, devicePixelRatio);
		fxLayer.particles.attach(c, innerWidth, innerHeight, devicePixelRatio, colors.resolve);
		window.addEventListener('resize', fit);
		return () => {
			window.removeEventListener('resize', fit);
			fxLayer.reset();
			fxLayer.particles.detach();
			colors.dispose();
		};
	});
	$effect(() => {
		let alive = true;
		void import('./FxOverlay.svelte').then((m) => alive && (Overlay = m.default));
		return () => {
			alive = false;
		};
	});
	// top layer: (re)show the popover so it stacks above any modal dialog opened since
	$effect(() => {
		void fxOverlay.raised;
		const el = root;
		if (!el?.showPopover) return;
		try {
			if (el.matches(':popover-open')) el.hidePopover();
			el.showPopover();
		} catch {
			/* popover unsupported: plain fixed layer (z-index) */
		}
	});
</script>

<div bind:this={root} class="fx-layer" popover="manual" aria-hidden="true" data-fx-layer>
	<canvas bind:this={canvas} class="fx-canvas"></canvas>
	<div class="fx-overlay" data-fx-overlay>
		{#if Overlay}<Overlay />{/if}
	</div>
</div>

<style>
	.fx-layer,
	.fx-canvas,
	.fx-overlay {
		position: fixed;
		inset: 0;
		pointer-events: none;
	}
	.fx-layer {
		/* top layer when popovers work; the fallback still stacks above modes and toasts' level */
		z-index: var(--z-toast);
		display: block;
		width: auto;
		height: auto;
		margin: 0;
		padding: 0;
		border: 0;
		overflow: visible;
		background: transparent;
		color: inherit;
	}
	.fx-canvas {
		width: 100%;
		height: 100%;
	}
</style>
