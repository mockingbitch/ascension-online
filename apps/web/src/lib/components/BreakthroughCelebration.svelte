<script lang="ts">
	import type { Aura } from '$lib/game/types';
	import FormationCircle from './FormationCircle.svelte';

	interface Props {
		realmName: string;
		stageName: string;
		aura: Aura;
		major: boolean;
		/** Tribulation name (e.g. "Lôi Kiếp") when the breakthrough was a tribulation. */
		tribulation?: string;
		onclose: () => void;
	}
	let { realmName, stageName, aura, major, tribulation, onclose }: Props = $props();

	// Auto-dismiss after ≤ 3s (graphic-design rule); click or Esc skips.
	$effect(() => {
		const t = setTimeout(onclose, major ? 3000 : 1800);
		const onKey = (e: KeyboardEvent) => e.key === 'Escape' && onclose();
		window.addEventListener('keydown', onKey);
		return () => {
			clearTimeout(t);
			window.removeEventListener('keydown', onKey);
		};
	});

	const sparks = Array.from({ length: 14 }, (_, i) => {
		const a = (i / 14) * Math.PI * 2;
		const r = 170 + (i % 3) * 40;
		return { x: Math.cos(a) * r, y: Math.sin(a) * r };
	});
</script>

<button
	type="button"
	class="overlay fixed inset-0 z-50 flex cursor-pointer flex-col items-center justify-center overflow-hidden"
	class:major
	style:--aura="var(--color-aura-{aura})"
	onclick={onclose}
	aria-label="Đóng"
>
	<span aria-hidden="true" class="pillar absolute inset-y-0 left-1/2"></span>
	<span aria-hidden="true" class="formation absolute"><FormationCircle color="var(--aura)" class="size-full" /></span>
	<span aria-hidden="true" class="shock absolute rounded-full"></span>
	{#if major}
		<svg aria-hidden="true" viewBox="0 0 200 600" class="bolt bolt-a absolute top-0" preserveAspectRatio="none">
			<path d="M110 0 L92 140 L120 150 L78 300 L104 306 L70 470 L96 476 L84 600" />
		</svg>
		<svg aria-hidden="true" viewBox="0 0 200 600" class="bolt bolt-b absolute top-0" preserveAspectRatio="none">
			<path d="M80 0 L104 120 L82 128 L118 260 L94 268 L124 420" />
		</svg>
		<span aria-hidden="true" class="flash absolute inset-0"></span>
	{/if}
	<span aria-hidden="true" class="absolute top-1/2 left-1/2">
		{#each sparks as s, i (i)}
			<span class="spark" style:--x="{s.x}px" style:--y="{s.y}px"></span>
		{/each}
	</span>

	<span class="caption relative text-sm font-medium tracking-[0.4em] text-gold-300 uppercase">
		{tribulation ? `Vượt qua ${tribulation}` : major ? 'Đột phá đại cảnh giới' : 'Đột phá thành công'}
	</span>
	<span class="title relative mt-4 font-display text-6xl font-bold sm:text-8xl">{realmName}</span>
	<span class="stage relative mt-3 font-display text-2xl text-mist-100">{stageName}</span>
	<span class="hint relative mt-10 text-xs text-mist-500">Chạm để tiếp tục</span>
	<span class="sr-only" aria-live="polite">Đột phá thành công: {realmName} {stageName}</span>
</button>

<style>
	.overlay {
		background: radial-gradient(circle at 50% 50%, color-mix(in oklab, var(--color-ink-800) 92%, transparent), color-mix(in oklab, var(--color-ink-950) 96%, transparent));
		animation: fade-in 300ms var(--ease-qi);
	}
	.pillar {
		width: 260px;
		transform: translateX(-50%);
		transform-origin: top;
		background: linear-gradient(90deg, transparent, color-mix(in oklab, var(--aura) 40%, transparent), color-mix(in oklab, white 55%, transparent), color-mix(in oklab, var(--aura) 40%, transparent), transparent);
		animation: pillar 1.6s var(--ease-qi) forwards;
	}
	@keyframes pillar {
		0% {
			transform: translateX(-50%) scaleY(0);
			opacity: 1;
		}
		35% {
			transform: translateX(-50%) scaleY(1);
			opacity: 1;
		}
		100% {
			transform: translateX(-50%) scaleY(1) scaleX(0.3);
			opacity: 0.15;
		}
	}
	.formation {
		width: min(560px, 90vw);
		aspect-ratio: 1;
		opacity: 0;
		filter: drop-shadow(0 0 10px var(--aura));
		animation: formation 2.6s var(--ease-qi) 0.15s forwards;
	}
	@keyframes formation {
		0% {
			transform: scale(0.35) rotate(-40deg);
			opacity: 0;
		}
		30% {
			opacity: 0.85;
		}
		100% {
			transform: scale(1.05) rotate(30deg);
			opacity: 0.35;
		}
	}
	.shock {
		width: 120px;
		height: 120px;
		border: 2px solid var(--aura);
		box-shadow: 0 0 30px var(--aura);
		opacity: 0;
		animation: shock 1.1s var(--ease-qi) 0.35s forwards;
	}
	@keyframes shock {
		0% {
			transform: scale(0.2);
			opacity: 1;
		}
		100% {
			transform: scale(9);
			opacity: 0;
		}
	}
	.bolt {
		height: 62%;
		width: 140px;
		fill: none;
		stroke: var(--color-mist-100);
		stroke-width: 3;
		stroke-linejoin: bevel;
		filter: drop-shadow(0 0 6px var(--color-spirit-cyan)) drop-shadow(0 0 18px var(--color-spirit-cyan));
		opacity: 0;
	}
	.bolt-a {
		left: calc(50% - 80px);
		animation: strike 0.5s steps(1) 0.1s forwards;
	}
	.bolt-b {
		left: calc(50% - 30px);
		height: 48%;
		animation: strike 0.45s steps(1) 0.45s forwards;
	}
	@keyframes strike {
		0% {
			opacity: 1;
		}
		20% {
			opacity: 0.2;
		}
		40% {
			opacity: 1;
		}
		100% {
			opacity: 0;
		}
	}
	.flash {
		background: var(--color-mist-100);
		opacity: 0;
		animation: flash 0.6s ease-out 0.1s;
	}
	@keyframes flash {
		0% {
			opacity: 0.45;
		}
		100% {
			opacity: 0;
		}
	}
	.spark {
		position: absolute;
		width: 5px;
		height: 5px;
		margin: -2.5px;
		border-radius: 50%;
		background: var(--aura);
		box-shadow: 0 0 10px var(--aura);
		opacity: 0;
		animation: burst 1.3s var(--ease-qi) 0.35s forwards;
	}
	@keyframes burst {
		0% {
			transform: translate(0, 0) scale(1.4);
			opacity: 1;
		}
		100% {
			transform: translate(var(--x), var(--y)) scale(0.2);
			opacity: 0;
		}
	}
	.caption {
		opacity: 0;
		animation: rise 600ms var(--ease-qi) 0.3s forwards;
	}
	.title {
		color: var(--aura);
		text-shadow:
			0 0 24px color-mix(in oklab, var(--aura) 60%, transparent),
			0 2px 0 var(--color-ink-950);
		opacity: 0;
		animation: reveal 900ms var(--ease-qi) 0.45s forwards;
	}
	.stage {
		opacity: 0;
		animation: rise 600ms var(--ease-qi) 0.8s forwards;
	}
	.hint {
		opacity: 0;
		animation: rise 600ms var(--ease-qi) 1.2s forwards;
	}
	@keyframes reveal {
		from {
			opacity: 0;
			letter-spacing: 0.5em;
			filter: blur(8px);
		}
		to {
			opacity: 1;
			letter-spacing: 0.04em;
			filter: blur(0);
		}
	}
	@keyframes rise {
		from {
			opacity: 0;
			transform: translateY(8px);
		}
		to {
			opacity: 1;
			transform: none;
		}
	}
	@keyframes fade-in {
		from {
			opacity: 0;
		}
	}
	/* reduced motion: global rule collapses durations; ensure final frame is legible */
	@media (prefers-reduced-motion: reduce) {
		.bolt,
		.flash {
			display: none;
		}
	}
</style>
