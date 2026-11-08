<!--
	Boot / loading screen when entering /play (spec §4.1): logo, a rotating formation ring, a real progress bar and a
	random đạo ngữ line. Steps: fonts ready, the first scene mounted (its art decoded), the initial game data. Leaves as
	soon as all are done (short minimum, hard limit — boot.ts). "Chạm để vào" (needsTapToEnter, boot.ts) only appears
	when the first scene has real music/ambience files to start, sound is on and audio is still locked: one-shot SFX
	(synth placeholders today) need no gate — the player's first real tap unlocks them. Muted players never see it. It
	never waits for audio files to load.
	Full: spinning ring + fade out · Lite: slow ring · Off: static, removed at once.
-->
<script lang="ts">
	import { onMount } from 'svelte';
	import { audio } from '$lib/engine/audio/AudioManager';
	import { bootDone, bootProgress, needsTapToEnter } from '$lib/engine/boot';
	import { sceneStage } from '$lib/engine/scene/stage.svelte';
	import { settings } from '$lib/engine/settings.svelte';
	import { randomDaoNgu } from '$lib/game/daoNgu';
	import { game } from '$lib/game/store.svelte';
	import FormationCircle from './FormationCircle.svelte';
	import Icon from './Icon.svelte';

	let { ondone }: { ondone: () => void } = $props();

	const line = randomDaoNgu();
	const steps = $state({ fonts: false, scene: false, data: false });
	const total = 3;
	const ready = $derived(Number(steps.fonts) + Number(steps.scene) + Number(steps.data));
	let needTap = $state(false);
	let leaving = $state(false);
	let root = $state<HTMLElement>();

	$effect(() => {
		if (game.ready) steps.data = true;
	});

	onMount(() => {
		const t0 = performance.now();
		let alive = true;
		void (document.fonts?.ready ?? Promise.resolve()).then(() => alive && (steps.fonts = true));
		// the scene the route shows: SceneLayer mounts it (def, view, art decoded)
		const waitScene = () => {
			if (!alive) return;
			const id = sceneStage.shown;
			if (!id) return void setTimeout(waitScene, 50);
			void sceneStage.whenMounted(id).then(() => alive && (steps.scene = true));
		};
		waitScene();
		const tick = setInterval(() => {
			if (!bootDone(ready, total, performance.now() - t0, settings.fx) || needTap || leaving) return;
			const a = audio();
			const def = sceneStage.def;
			if (needsTapToEnter({ unlocked: a.unlocked, muted: settings.audio.muted, loopFile: a.hasFile(def?.music) || a.hasFile(def?.ambience) })) {
				needTap = true;
				return;
			}
			leave();
		}, 50);
		return () => {
			alive = false;
			clearInterval(tick);
		};
	});

	function leave() {
		if (leaving) return;
		leaving = true;
		if (settings.fx === 'off' || !root) return ondone();
		const a = root.animate([{ opacity: 1 }, { opacity: 0 }], { duration: settings.fx === 'full' ? 380 : 180, easing: 'ease-out', fill: 'forwards' });
		a.finished.then(ondone, ondone);
	}
</script>

<div bind:this={root} class="boot" role="status" aria-live="polite" aria-label="Đang vào Tiên Đạo" data-boot data-boot-ready={ready === total}>
	<div class="mark" aria-hidden="true">
		<span class="ring"><FormationCircle color="var(--color-gold-500)" class="size-full" /></span>
		<span class="glyph"><Icon name="lotus" size={44} stroke={1.3} /></span>
	</div>
	<h1 class="font-display text-4xl font-bold text-gold-300">Tiên Đạo</h1>
	<div class="bar" role="progressbar" aria-label="Đang tải" aria-valuemin={0} aria-valuemax={total} aria-valuenow={ready}>
		<span class="fill" style:transform="scaleX({bootProgress(ready, total)})"></span>
	</div>
	<p class="line font-display">{line}</p>
	{#if needTap}
		<button type="button" class="btn btn-gold mt-2" onclick={leave} data-boot-tap>Chạm để vào</button>
	{/if}
</div>

<style>
	.boot {
		position: fixed;
		inset: 0;
		z-index: var(--z-boot);
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 1rem;
		padding: 1.5rem;
		text-align: center;
		background:
			var(--grain),
			radial-gradient(ellipse at 50% 40%, var(--color-ink-800), var(--color-ink-950) 70%);
	}
	.mark {
		position: relative;
		display: grid;
		width: 9rem;
		height: 9rem;
		place-items: center;
	}
	.ring {
		position: absolute;
		inset: 0;
	}
	:global(:root[data-fx='full']) .ring {
		animation: spin 9s linear infinite;
	}
	:global(:root[data-fx='lite']) .ring {
		animation: spin 24s linear infinite;
	}
	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}
	.glyph {
		color: var(--color-gold-300);
		filter: drop-shadow(0 0 10px color-mix(in oklab, var(--color-gold-300) 55%, transparent));
	}
	.bar {
		width: min(16rem, 70vw);
		height: 4px;
		overflow: hidden;
		border-radius: 9999px;
		background: var(--color-ink-950);
		box-shadow: inset 0 0 0 1px color-mix(in oklab, var(--color-gold-500) 30%, transparent);
	}
	.fill {
		display: block;
		height: 100%;
		transform-origin: 0 50%;
		background: linear-gradient(90deg, var(--color-jade-500), var(--color-gold-300));
		transition: transform var(--dur-base) var(--ease-qi);
	}
	.line {
		max-width: 26rem;
		font-size: 1.1rem;
		font-style: italic;
		color: var(--color-mist-300);
	}
</style>
