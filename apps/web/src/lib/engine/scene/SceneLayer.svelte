<!--
	SceneLayer (spec §3.1): always mounted behind the game UI. Renders the stage's current scene through the
	SceneRenderer (lazy: renderer, def and view are dynamic imports) into a box that follows the page's SceneSlot, or
	the shell's stage cell when the page has none. It never unmounts for a window: a covering window or screen pauses
	the renderer (parallax) and the ambient loops, and dims the scene.
-->
<script lang="ts">
	import { onDestroy, tick, untrack } from 'svelte';
	import { tokenResolver } from '../fx/colors';
	import { ParticleField } from '../fx/layer';
	import { ParticleSystem } from '../fx/particles';
	import { createRenderer } from '../render';
	import type { SceneRenderer } from '../render/types';
	import { settings } from '../settings.svelte';
	import { loadSceneDef } from './defs';
	import type { SceneStage } from './stage.svelte';
	import type { SceneId } from './types';
	import { SceneViewCtx, type SceneView } from './view.svelte';
	import { loadSceneView } from './views';

	let { stage }: { stage: SceneStage } = $props();

	let host = $state<HTMLDivElement>();
	let renderer = $state.raw<SceneRenderer | null>(null);
	let current = $state.raw<{ id: SceneId; View: SceneView | null; ctx: SceneViewCtx } | null>(null);
	let box = $state({ x: 0, y: 0, w: 0, h: 0 });
	let canvas = $state<HTMLCanvasElement>();
	// ambient particles of the scene (motes, petals, smoke): a small pool under the UI, separate from the FxLayer
	const ambient = new ParticleField(new ParticleSystem(settings.fx, Math.random, { full: 90, lite: 30 }));
	$effect(() => ambient.setQuality(settings.fx));
	$effect(() => ambient.setPaused(stage.covered || stage.phase !== 'idle'));
	$effect(() => {
		const c = canvas;
		if (!c) return;
		const colors = tokenResolver();
		ambient.particles.attach(c, untrack(() => box.w), untrack(() => box.h), devicePixelRatio, colors.resolve);
		return () => {
			ambient.reset();
			ambient.particles.detach();
			colors.dispose();
		};
	});
	$effect(() => {
		if (canvas) ambient.particles.resize(box.w, box.h, devicePixelRatio);
	});

	// The box follows the slot (or the cell): resize, window resize and any scroll (capture: inner scrollers too).
	$effect(() => {
		const target = stage.slot ?? stage.cell;
		if (!target) return;
		const measure = () => {
			const r = target.getBoundingClientRect();
			if (r.left !== box.x || r.top !== box.y || r.width !== box.w || r.height !== box.h) box = { x: r.left, y: r.top, w: r.width, h: r.height };
		};
		measure();
		const ro = new ResizeObserver(measure);
		ro.observe(target);
		if (stage.cell && stage.cell !== target) ro.observe(stage.cell);
		window.addEventListener('resize', measure);
		document.addEventListener('scroll', measure, { capture: true, passive: true });
		return () => {
			ro.disconnect();
			window.removeEventListener('resize', measure);
			document.removeEventListener('scroll', measure, { capture: true });
		};
	});

	// Mount the shown scene (under the transition cover when there is one), then tell the stage it is ready.
	$effect(() => {
		const id = stage.shown;
		const el = host;
		if (!id || !el) return;
		let cancelled = false;
		void (async () => {
			const r = untrack(() => renderer);
			const [rr, def, View] = await Promise.all([r ?? createRenderer(settings.renderer), loadSceneDef(id), loadSceneView(id)]);
			if (cancelled || !def) return;
			untrack(() => current)?.ctx.dispose();
			current = null;
			await rr.mount(el, def, { quality: untrack(() => settings.fx) });
			if (cancelled) return;
			renderer = rr;
			stage.renderer = rr;
			stage.def = def;
			current = { id, View, ctx: new SceneViewCtx(rr, def, el, ambient) };
			await tick();
			stage.mounted(id);
		})();
		return () => {
			cancelled = true;
		};
	});

	$effect(() => renderer?.setQuality(settings.fx));
	$effect(() => {
		if (!renderer) return;
		if (stage.covered || stage.phase !== 'idle') renderer.pause();
		else renderer.resume();
	});

	onDestroy(() => {
		current?.ctx.dispose();
		renderer?.unmount();
		stage.renderer = null;
		stage.def = null;
	});
</script>

<div class="scene-layer" class:is-covered={stage.covered} data-quality={settings.fx} aria-hidden={stage.covered ? 'true' : undefined}>
	<div
		class="box"
		data-scene={stage.shown}
		style:left="{box.x}px"
		style:top="{box.y}px"
		style:width="{box.w}px"
		style:height="{box.h}px"
	>
		<div bind:this={host} class="art-host"></div>
		{#if current?.View}
			{@const View = current.View}
			<div class="view" data-scene-view={current.id}>
				<View scene={current.ctx} quality={settings.fx} phase={stage.phase} covered={stage.covered} />
			</div>
		{/if}
		<canvas bind:this={canvas} class="ambient" aria-hidden="true" data-scene-particles></canvas>
	</div>
	<div class="dim" aria-hidden="true"></div>
</div>

<style>
	.scene-layer {
		position: fixed;
		inset: 0;
		z-index: 0;
		overflow: hidden;
		background: radial-gradient(ellipse at 50% 30%, var(--color-ink-850), var(--color-ink-950) 75%);
	}
	.box {
		position: absolute;
		overflow: hidden;
		contain: layout paint;
	}
	.box[data-scene='tu_tieu_cung'] {
		background: radial-gradient(ellipse at 50% 20%, color-mix(in oklab, var(--color-spirit-violet) 30%, var(--color-ink-900)), var(--color-ink-950) 70%);
	}
	.box[data-scene='dong_phu'] {
		background: radial-gradient(ellipse at 50% 62%, color-mix(in oklab, var(--color-jade-700) 28%, var(--color-ink-900)), var(--color-ink-950) 72%);
	}
	.art-host,
	.view,
	.ambient {
		position: absolute;
		inset: 0;
	}
	.ambient {
		width: 100%;
		height: 100%;
		pointer-events: none;
	}
	.view {
		pointer-events: none;
	}
	.dim {
		position: absolute;
		inset: 0;
		pointer-events: none;
		background: var(--color-ink-950);
		opacity: 0;
		transition: opacity var(--dur-base) var(--ease-qi);
	}
	.is-covered .dim {
		opacity: 0.35;
	}
	/* covered: ambient loops hold still (the renderer pauses its parallax separately) */
	.is-covered :global(*) {
		animation-play-state: paused !important;
	}
</style>
