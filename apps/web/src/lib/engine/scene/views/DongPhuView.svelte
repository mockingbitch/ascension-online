<!--
	Động Phủ scene overlay: the cultivator meditating on the floor line, the realm-coloured aura, qi motes converging
	on the đan điền, cosmetic decorations and the gold pillar at a bình cảnh — every point from the dong_phu def's
	anchors. Ambient (§4.3): the cultivator breathes (scale 1 → 1.015), the aura pulses, linh khí motes converge on the
	đan điền and — on days the player offered incense (server: the incense task's streak.offeredToday) — smoke rises
	from the burner. Motes and smoke are particles of the scene field (paused while covered).
	Full: breathing + aura + motes ~4/s + smoke · Lite: aura + motes ~1.4/s + thin smoke · Off: static (a lit burner
	still glows). Values are server state (game store).
-->
<script lang="ts">
	import CosmeticArt from '$lib/components/CosmeticArt.svelte';
	import CultivatorFigure from '$lib/components/CultivatorFigure.svelte';
	import FormationCircle from '$lib/components/FormationCircle.svelte';
	import { art } from '$lib/game/art';
	import { cosmetics, hangs, slugOf } from '$lib/game/cosmetics.svelte';
	import { useIdleClock } from '$lib/game/idle.svelte';
	import { game } from '$lib/game/store.svelte';
	import { mote, smoke } from '$lib/engine/fx/presets';
	import { every, type SceneViewProps } from '../view.svelte';

	let { scene, quality }: SceneViewProps = $props();

	const clock = useIdleClock();
	const aura = $derived(game.catalogue?.realmOf(game.character?.stageId ?? '')?.visual.aura ?? 'jade');
	const ready = $derived(clock.atBottleneck);
	const meditate = art('characters/meditate');

	// Wall-hung pieces take the upper anchors, floor pieces the lower ones (same rule as CultivationScene).
	const placed = $derived.by(() => {
		const wall: string[] = [];
		const floor: string[] = [];
		for (const id of game.character?.appearance?.caveDecorationIds ?? []) (hangs(slugOf(id)) ? wall : floor).push(id);
		while (wall.length > 2) floor.push(wall.pop()!);
		while (floor.length > 2 && wall.length < 2) wall.push(floor.pop()!);
		return [
			...wall.map((id, i) => ({ id, anchor: i === 0 ? 'deco.wall_left' : 'deco.wall_right', small: false })),
			...floor.map((id, i) => ({ id, anchor: i === 0 ? 'deco.floor_left' : i === 1 ? 'deco.floor_right' : 'deco.floor_centre', small: i > 1 }))
		];
	});

	const incenseLit = $derived(!!game.board?.tasks.some((t) => t.kind === 'incense' && (t.streak?.offeredToday || t.claimed)));

	const base = $derived(scene.at('cultivator.base'));
	const centre = $derived(scene.at('cultivator'));
	const dantian = $derived(scene.at('dantian'));
	const top = $derived(scene.project({ x: 0.5, y: 0 }));
	const burner = $derived(scene.at('incense'));
	// motes take the realm aura colour (token)
	const auraMote = $derived({ ...mote, colors: [`var(--color-aura-${aura})`, 'var(--color-spirit-cyan)'] });

	// linh khí converging: each mote starts on a ring around the đan điền and curves in (path particles)
	$effect(() => {
		let k = 0;
		return scene.fx.addEmitter(
			every(
				() => (quality === 'full' ? 250 : quality === 'lite' ? 700 : 0),
				() => {
					const d = dantian;
					if (!d) return;
					const u = scene.unit;
					const a = (k++ * 2.39996) % (Math.PI * 2); // golden-angle walk around the ring
					const r = u * (30 + (k % 4) * 4.5);
					const from = { x: d.x + Math.cos(a) * r, y: d.y + Math.sin(a) * r * 0.7 };
					const ctrl = { x: d.x + Math.cos(a + 0.9) * r * 0.55, y: d.y + Math.sin(a + 0.9) * r * 0.4 };
					scene.fx.path(auraMote, { from, to: d, ctrl, ms: 3000 }, 1);
				}
			)
		);
	});
	// incense smoke rising from the burner (only when offered today)
	$effect(() => {
		if (!incenseLit) return;
		return scene.fx.addEmitter(
			every(
				() => (quality === 'full' ? 380 : quality === 'lite' ? 1100 : 0),
				() => {
					const b = burner;
					if (b) scene.fx.burst(smoke, b.x, b.y - scene.unit * 5.5, 1);
				}
			)
		);
	});
</script>

<div class="cave" data-quality={quality} style:--u="{scene.unit}px" style:--aura="var(--color-aura-{aura})">
	{#if centre}
		<div class="halo" style:left="{centre.x}px" style:top="{centre.y}px" aria-hidden="true"></div>
	{/if}

	{#each placed as d (d.id)}
		{@const c = cosmetics.get(d.id)}
		{@const p = scene.at(d.anchor)}
		{#if p}
			<div class="deco" class:is-small={d.small} style:left="{p.x}px" style:top="{p.y}px" title={c?.name}>
				<CosmeticArt kind="cave_decoration" slug={slugOf(d.id)} rarity={c?.rarity} size={88} />
			</div>
		{/if}
	{/each}

	{#if burner}
		<div class="burner" class:is-lit={incenseLit} style:left="{burner.x}px" style:top="{burner.y}px" aria-hidden="true" data-incense-burner>
			<span class="sticks"></span>
			<span class="bowl"></span>
		</div>
	{/if}

	{#if ready && base && top}
		<div class="pillar" style:left="{base.x}px" style:top="{top.y}px" style:height="{base.y - top.y}px" aria-hidden="true"></div>
	{/if}

	{#key game.backlash}
		{#if base}
			<div class="figure" class:shake={game.backlash > 0} style:left="{base.x}px" style:top="{base.y}px">
				{#if meditate}
					<img src={meditate.src} srcset={meditate.srcset} alt="" class="art" draggable="false" />
				{:else}
					<div class="floor" aria-hidden="true"><div class="spin"><FormationCircle color={ready ? 'var(--color-gold-300)' : 'var(--aura)'} class="size-full" /></div></div>
					<CultivatorFigure {aura} class="fallback-figure" />
				{/if}
			</div>
		{/if}
		{#if game.backlash > 0 && dantian}
			<div class="backlash" style:left="{dantian.x}px" style:top="{dantian.y}px" aria-hidden="true"></div>
		{/if}
	{/key}

</div>

<style>
	.cave {
		position: absolute;
		inset: 0;
	}
	.halo,
	.deco,
	.pillar,
	.figure,
	.burner,
	.backlash {
		position: absolute;
	}
	.halo {
		width: calc(var(--u) * 100);
		aspect-ratio: 1;
		transform: translate(-50%, -50%);
		background:
			radial-gradient(circle at 50% 50%, color-mix(in oklab, var(--aura) 34%, transparent) 0, transparent 26%),
			radial-gradient(ellipse at 50% 64%, color-mix(in oklab, var(--aura) 18%, transparent) 0, transparent 42%);
		opacity: 0.8;
	}
	[data-quality='full'] .halo,
	[data-quality='lite'] .halo {
		animation: var(--animate-qi-breathe);
	}
	.deco {
		width: calc(var(--u) * 20);
		aspect-ratio: 1;
		display: grid;
		place-items: center;
		transform: translate(-50%, -50%);
		opacity: 0.92;
		filter: drop-shadow(0 4px 10px var(--color-ink-950));
	}
	.deco.is-small {
		width: calc(var(--u) * 15);
	}
	.deco :global(.cart) {
		width: 100% !important;
		height: 100% !important;
	}
	.pillar {
		width: calc(var(--u) * 34);
		transform: translateX(-50%);
		background: linear-gradient(90deg, transparent, color-mix(in oklab, var(--color-gold-300) 22%, transparent), transparent);
		mask-image: linear-gradient(180deg, transparent, var(--color-ink-950) 25%, var(--color-ink-950) 75%, transparent);
	}
	[data-quality='full'] .pillar {
		animation: pillar 2.8s var(--ease-in-out-sine) infinite;
	}
	@keyframes pillar {
		0%, 100% { opacity: 0.55; transform: translateX(-50%) scaleX(0.85); }
		50% { opacity: 1; transform: translateX(-50%) scaleX(1); }
	}
	/* the figure stands on the floor line: 92% of the 4:3 stage tall = 69% of its width */
	.figure {
		height: calc(var(--u) * 69);
		aspect-ratio: 1;
		transform: translate(-50%, -100%);
		transform-origin: 50% 100%;
	}
	[data-quality='full'] .figure:not(.shake) {
		animation: breathe 4.6s var(--ease-in-out-sine) infinite;
	}
	@keyframes breathe {
		0%, 100% { transform: translate(-50%, -100%) scale(1); }
		50% { transform: translate(-50%, -100%) scale(1.015); }
	}
	.art {
		width: 100%;
		height: 100%;
		object-fit: contain;
		object-position: bottom;
		filter: drop-shadow(0 0 calc(var(--u) * 3) color-mix(in oklab, var(--aura) 45%, transparent));
	}
	.floor {
		position: absolute;
		left: 50%;
		bottom: -32%;
		width: 125%;
		aspect-ratio: 1;
		transform: translateX(-50%) scaleY(0.3);
		opacity: 0.75;
	}
	.spin {
		width: 100%;
		height: 100%;
	}
	[data-quality='full'] .spin {
		animation: spin 80s linear infinite;
	}
	@keyframes spin {
		to { transform: rotate(360deg); }
	}
	.figure :global(.fallback-figure) {
		position: absolute;
		inset: 10% 0 8%;
		margin: auto;
		height: 82%;
	}
	.shake {
		animation: shake 420ms ease-out;
	}
	@keyframes shake {
		0%, 100% { transform: translate(-50%, -100%); }
		20% { transform: translate(calc(-50% - 6px), -100%); }
		40% { transform: translate(calc(-50% + 5px), -100%); }
		60% { transform: translate(calc(-50% - 3px), -100%); }
		80% { transform: translate(calc(-50% + 2px), -100%); }
	}
	.burner {
		width: calc(var(--u) * 6);
		height: calc(var(--u) * 6);
		transform: translate(-50%, -100%);
	}
	.burner .bowl {
		position: absolute;
		left: 0;
		right: 0;
		bottom: 0;
		height: 40%;
		border-radius: 10% 10% 50% 50%;
		background: linear-gradient(180deg, var(--color-gold-500), var(--color-gold-700));
		box-shadow: 0 2px 6px var(--color-ink-950);
	}
	.burner .sticks {
		position: absolute;
		left: 30%;
		right: 30%;
		bottom: 38%;
		height: 55%;
		background: repeating-linear-gradient(90deg, var(--color-cinnabar-500) 0 2px, transparent 2px 40%);
	}
	.burner.is-lit .sticks::before {
		content: '';
		position: absolute;
		left: -20%;
		right: -20%;
		top: -12%;
		height: 18%;
		border-radius: 9999px;
		background: radial-gradient(ellipse, var(--color-gold-300), transparent 70%);
	}
	.backlash {
		width: calc(var(--u) * 110);
		aspect-ratio: 1;
		margin: calc(var(--u) * -55);
		pointer-events: none;
		background: radial-gradient(circle, color-mix(in oklab, var(--color-cinnabar-500) 55%, transparent), transparent 50%);
		opacity: 0;
		animation: backlash 1.1s ease-out;
	}
	@keyframes backlash {
		0% { opacity: 0; transform: scale(0.6); }
		15% { opacity: 1; }
		100% { opacity: 0; transform: scale(1.4); }
	}
</style>
