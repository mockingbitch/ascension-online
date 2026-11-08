<!--
	Opening "video" placeholder (cot-truyen §10): the 8 scenes as still frames with captions, composed from the shipped
	raster art + CSS. Auto-advances (display only) unless the player prefers reduced motion, then it waits for a tap.
	Skippable at any time (button, Esc); tap / → / Space = next scene, ← = previous; a pause control stops the timer.
-->
<script lang="ts">
	import { onDestroy } from 'svelte';
	import { MediaQuery } from 'svelte/reactivity';
	import { fade } from 'svelte/transition';
	import { art } from '$lib/game/art';
	import { factionColor } from '$lib/game/faction.svelte';
	import { openingScenes, sceneMs } from '$lib/game/onboarding';
	import type { FactionView } from '$lib/game/types';
	import FactionEmblem from './FactionEmblem.svelte';
	import Icon from './Icon.svelte';

	interface Props {
		/** starting factions, for the "đại năng hiện thế" frame (emblems only, no names) */
		factions: FactionView[];
		onfinish: () => void;
	}
	let { factions, onfinish }: Props = $props();

	const map = art('world/map');
	const dawn = art('ui/splash');
	const hall = art('scenes/tu-tieu-cung');
	const sage = art('characters/hong-quan');
	const reduced = new MediaQuery('(prefers-reduced-motion: reduce)');

	let index = $state(0);
	let paused = $state(false);
	const scene = $derived(openingScenes[index]!);
	const total = openingScenes.length;
	const ms = $derived(sceneMs(scene.caption));
	const auto = $derived(!reduced.current && !paused);
	// Svelte transitions run on the Web Animations API, which the global reduced-motion CSS rule doesn't reach.
	const dur = (ms: number) => (reduced.current ? 0 : ms);

	// One timeout for the whole sequence; restarts whenever the scene or the auto state changes.
	let timer: ReturnType<typeof setTimeout> | undefined;
	$effect(() => {
		clearTimeout(timer);
		if (auto) timer = setTimeout(next, ms);
		return () => clearTimeout(timer);
	});
	onDestroy(() => clearTimeout(timer));

	function next() {
		if (index >= total - 1) onfinish();
		else index += 1;
	}
	function prev() {
		if (index > 0) index -= 1;
	}
	function onkey(e: KeyboardEvent) {
		if (e.key === 'Escape') onfinish();
		else if (e.key === 'ArrowRight' || e.key === ' ' || e.key === 'Enter') {
			e.preventDefault();
			next();
		} else if (e.key === 'ArrowLeft') prev();
	}
</script>

<svelte:window onkeydown={onkey} />

<section class="opening fixed inset-0 z-(--z-mode) overflow-hidden bg-ink-950 text-mist-100" aria-roledescription="trình chiếu" aria-label="Mở đầu: lịch sử Hồng Hoang">
	{#key index}
		<div class="frame absolute inset-0" in:fade={{ duration: dur(700) }} out:fade={{ duration: dur(500) }} aria-hidden="true">
			{#if scene.visual === 'chaos'}
				<div class="chaos absolute inset-0"><span class="swirl"></span><span class="sleeper"></span></div>
			{:else if scene.visual === 'split'}
				<div class="split absolute inset-0">
					<span class="sky"></span><span class="earth"></span>
					<svg class="crack" viewBox="0 0 400 40" preserveAspectRatio="none">
						<polyline points="0,22 60,18 95,25 140,14 190,24 230,16 280,23 330,15 365,21 400,18" />
					</svg>
				</div>
			{:else if scene.visual === 'world'}
				<div class="still world absolute inset-0">
					{#if map}<img src={map.src} srcset={map.srcset} alt="" class="kb" draggable="false" />{/if}
					<span class="sun"></span><span class="moon"></span>
				</div>
			{:else if scene.visual === 'dawn'}
				<div class="still dawn absolute inset-0">
					{#if dawn}<img src={dawn.src} srcset={dawn.srcset} alt="" class="kb" draggable="false" />{/if}
				</div>
			{:else if scene.visual === 'beasts'}
				<div class="beasts absolute inset-0">
					<span class="orb" style:--c="var(--color-aura-jade)" style:--x="30%" style:--y="38%">Long</span>
					<span class="orb" style:--c="var(--color-aura-crimson)" style:--x="70%" style:--y="34%">Phượng</span>
					<span class="orb" style:--c="var(--color-aura-golden)" style:--x="50%" style:--y="52%">Kỳ Lân</span>
					<span class="ash"></span>
				</div>
			{:else if scene.visual === 'powers'}
				<div class="powers absolute inset-0">
					<ul class="emblems">
						{#each factions as f, i (f.id)}
							<li style:--fc={factionColor(f)} style:--d="{i * 260}ms"><FactionEmblem id={f.id} size={96} /></li>
						{/each}
					</ul>
				</div>
			{:else if scene.visual === 'sage'}
				<div class="still sage absolute inset-0">
					{#if hall}<img src={hall.src} srcset={hall.srcset} alt="" class="bg" draggable="false" />{/if}
					<span class="halo"></span>
					{#if sage}<img src={sage.src} srcset={sage.srcset} alt="" class="figure" draggable="false" />{/if}
				</div>
			{:else}
				<div class="still hall absolute inset-0">
					{#if hall}<img src={hall.src} srcset={hall.srcset} alt="" class="kb toward-dais" draggable="false" />{/if}
				</div>
			{/if}
			<span class="scrim absolute inset-0"></span>
		</div>
	{/key}

	<!-- tap anywhere (outside the controls) = next scene -->
	<button type="button" class="absolute inset-0 z-(--z-local-1) cursor-pointer" aria-label="Cảnh tiếp theo" onclick={next}></button>

	<header class="pointer-events-none absolute inset-x-0 top-0 z-(--z-local-2) px-4 pt-[max(1rem,env(safe-area-inset-top))] sm:px-8">
		<ol class="flex gap-1.5" aria-label="Tiến trình">
			{#each openingScenes as s, i (s.title)}
				<li class="bar" aria-current={i === index ? 'step' : undefined}>
					<span
						class="fill"
						class:done={i < index}
						class:running={i === index && auto}
						class:held={i === index && !auto}
						style:--ms="{ms}ms"
					></span>
					<span class="sr-only">{s.title}</span>
				</li>
			{/each}
		</ol>
		<div class="pointer-events-auto mt-3 flex items-center justify-end gap-2">
			{#if !reduced.current}
				<button type="button" class="btn btn-ghost h-10 px-3 text-sm" onclick={() => (paused = !paused)} aria-pressed={paused}>
					{paused ? 'Xem tiếp' : 'Tạm dừng'}
				</button>
			{/if}
			<button type="button" class="btn btn-ghost h-10 px-3 text-sm" onclick={onfinish}>
				Bỏ qua <Icon name="arrow" size={16} />
			</button>
		</div>
	</header>

	<footer class="pointer-events-none absolute inset-x-0 bottom-0 z-(--z-local-2) px-5 pb-[max(1.5rem,env(safe-area-inset-bottom))] sm:px-10 sm:pb-10">
		{#key index}
			<div class="mx-auto max-w-3xl" in:fade={{ duration: dur(600), delay: dur(250) }} aria-live="polite">
				<p class="eyebrow">Cảnh {index + 1} / {total}</p>
				<h2 class="mt-1 font-display text-3xl font-bold text-gold-300 sm:text-5xl">{scene.title}</h2>
				<p class="mt-2 text-base leading-relaxed text-mist-100 sm:text-lg">{scene.caption}</p>
			</div>
		{/key}
		<div class="pointer-events-auto mx-auto mt-4 flex max-w-3xl items-center gap-2">
			<button type="button" class="btn btn-ghost h-11 w-11 p-0" disabled={index === 0} onclick={prev} aria-label="Cảnh trước">
				<Icon name="arrow" size={18} class="rotate-180" />
			</button>
			<button type="button" class="btn btn-jade h-11 flex-1 sm:flex-none sm:px-6" onclick={next}>
				{index === total - 1 ? 'Bước vào Tử Tiêu Cung' : 'Tiếp'}
			</button>
		</div>
	</footer>
</section>

<style>
	.scrim {
		background:
			linear-gradient(180deg, color-mix(in oklab, var(--color-ink-950) 70%, transparent), transparent 22%),
			linear-gradient(0deg, var(--color-ink-950) 2%, color-mix(in oklab, var(--color-ink-950) 80%, transparent) 30%, transparent 62%);
	}
	/* stills: slow push-in (Ken Burns), transform only */
	.still img {
		position: absolute;
		inset: 0;
		width: 100%;
		height: 100%;
		object-fit: cover;
	}
	.kb {
		animation: push 12s var(--ease-qi) both;
	}
	.toward-dais {
		transform-origin: 50% 22%;
	}
	@keyframes push {
		from {
			transform: scale(1.02);
		}
		to {
			transform: scale(1.14);
		}
	}
	.dawn img {
		object-position: 50% 42%;
	}

	/* 1 · Hỗn Độn */
	.chaos {
		background: radial-gradient(ellipse at 50% 45%, var(--color-ink-800), var(--color-ink-950) 70%);
	}
	.swirl {
		position: absolute;
		inset: -40%;
		background: conic-gradient(
			from 0deg,
			transparent,
			color-mix(in oklab, var(--color-spirit-violet) 30%, transparent),
			transparent 30%,
			color-mix(in oklab, var(--color-ink-600) 60%, transparent),
			transparent 60%,
			color-mix(in oklab, var(--color-spirit-cyan) 18%, transparent),
			transparent
		);
		filter: blur(40px);
		animation: spin 40s linear infinite;
	}
	.sleeper {
		position: absolute;
		left: 50%;
		top: 42%;
		width: 9rem;
		height: 9rem;
		translate: -50% -50%;
		border-radius: 50%;
		background: radial-gradient(circle, color-mix(in oklab, var(--color-gold-300) 45%, transparent), transparent 70%);
		animation: var(--animate-qi-breathe);
	}
	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}

	/* 2 · Khai thiên: sky rises, earth sinks, the crack glows */
	.split .sky,
	.split .earth {
		position: absolute;
		inset-inline: 0;
		height: 50%;
	}
	.split .sky {
		top: 0;
		background:
			radial-gradient(ellipse 60% 45% at 50% 100%, color-mix(in oklab, var(--color-gold-300) 55%, transparent), transparent),
			linear-gradient(
			180deg,
			color-mix(in oklab, var(--color-mist-100) 70%, var(--color-jade-300)),
			color-mix(in oklab, var(--color-mist-300) 45%, var(--color-ink-700)) 70%,
			color-mix(in oklab, var(--color-gold-300) 40%, var(--color-ink-700))
		);
		animation: rise 3s var(--ease-qi) both;
	}
	.split .earth {
		bottom: 0;
		background: linear-gradient(180deg, var(--color-ink-900), var(--color-ink-950));
		animation: sink 3s var(--ease-qi) both;
	}
	.crack {
		position: absolute;
		left: 0;
		top: calc(48% - 2rem);
		width: 100%;
		height: 4rem;
		fill: none;
		stroke: var(--color-gold-300);
		stroke-width: 2.5;
		filter: drop-shadow(0 0 10px var(--color-gold-500));
	}
	@keyframes rise {
		to {
			transform: translateY(-7%);
		}
	}
	@keyframes sink {
		to {
			transform: translateY(7%);
		}
	}

	/* 3 · Bàn Cổ vẫn lạc: the map with a sun and a moon */
	.world img {
		filter: saturate(0.8) brightness(0.8);
	}
	.sun,
	.moon {
		position: absolute;
		top: 14%;
		width: 3.5rem;
		height: 3.5rem;
		border-radius: 50%;
	}
	.sun {
		left: 14%;
		background: radial-gradient(circle, var(--color-gold-300), color-mix(in oklab, var(--color-gold-500) 0%, transparent) 70%);
	}
	.moon {
		right: 14%;
		background: radial-gradient(circle, var(--color-mist-100), color-mix(in oklab, var(--color-mist-300) 0%, transparent) 70%);
		opacity: 0.8;
	}

	/* 5 · Long Hán Sơ Kiếp: three auras that dim into ash */
	.beasts {
		background: radial-gradient(ellipse at 50% 45%, var(--color-ink-700), var(--color-ink-950) 75%);
	}
	.orb {
		position: absolute;
		left: var(--x);
		top: var(--y);
		translate: -50% -50%;
		display: grid;
		place-items: center;
		width: min(42vw, 15rem);
		aspect-ratio: 1;
		border-radius: 50%;
		background: radial-gradient(circle, color-mix(in oklab, var(--c) 70%, transparent), transparent 68%);
		font-family: var(--font-display);
		font-size: clamp(1.4rem, 4vw, 2.2rem);
		font-weight: 700;
		color: var(--color-mist-100);
		text-shadow: 0 0 12px var(--c);
		animation: wane 7s var(--ease-qi) both;
	}
	.ash {
		position: absolute;
		inset: 0;
		background: var(--grain), linear-gradient(0deg, color-mix(in oklab, var(--color-ink-950) 60%, transparent), transparent);
	}
	@keyframes wane {
		from {
			opacity: 1;
			transform: scale(1);
		}
		to {
			opacity: 0.75;
			transform: scale(0.9);
		}
	}

	/* 6 · Các đại năng xuất hiện: faction emblems appear one by one */
	.powers {
		background:
			var(--grain),
			radial-gradient(ellipse at 50% 40%, var(--color-ink-800), var(--color-ink-950) 72%);
	}
	.emblems {
		position: absolute;
		inset-inline: 0;
		top: 26%;
		display: flex;
		flex-wrap: wrap;
		justify-content: center;
		gap: clamp(0.75rem, 4vw, 2.5rem);
		padding-inline: 1rem;
	}
	.emblems li {
		color: var(--fc);
		filter: drop-shadow(0 0 14px var(--fc));
		animation: appear 900ms var(--ease-qi) both;
		animation-delay: var(--d);
	}
	@media (prefers-reduced-motion: reduce) {
		.emblems li {
			animation-delay: 0s;
		}
	}
	@keyframes appear {
		from {
			opacity: 0;
			transform: translateY(12px) scale(0.9);
		}
	}

	/* 7 · Hồng Quân hiện thế */
	.sage .bg {
		filter: blur(6px) brightness(0.35);
	}
	.halo {
		position: absolute;
		left: 50%;
		top: 40%;
		width: min(90vw, 34rem);
		aspect-ratio: 1;
		translate: -50% -50%;
		border-radius: 50%;
		background: radial-gradient(circle, color-mix(in oklab, var(--color-spirit-violet) 40%, transparent), transparent 65%);
		animation: var(--animate-qi-breathe);
	}
	.sage .figure {
		inset: auto;
		left: 50%;
		top: 40%;
		width: min(78vw, 26rem);
		height: auto;
		translate: -50% -50%;
		object-fit: contain;
		animation: appear 1.2s var(--ease-qi) both;
	}

	/* progress */
	.bar {
		position: relative;
		flex: 1;
		height: 3px;
		overflow: hidden;
		border-radius: 2px;
		background: color-mix(in oklab, var(--color-mist-100) 22%, transparent);
	}
	.fill {
		position: absolute;
		inset: 0;
		transform-origin: left;
		transform: scaleX(0);
		background: var(--color-gold-300);
	}
	.fill.done,
	.fill.held {
		transform: scaleX(1);
	}
	.fill.held {
		background: color-mix(in oklab, var(--color-gold-300) 60%, transparent);
	}
	.fill.running {
		animation: progress var(--ms) linear both;
	}
	@keyframes progress {
		to {
			transform: scaleX(1);
		}
	}
</style>
