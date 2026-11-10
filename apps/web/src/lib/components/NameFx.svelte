<!-- A character name (or any text) with its chat name effect (cosmetics.yaml chat_aura:*). Looks only; the gradient/
     glow is decorative. Each effect is distinct: Mặc Hương an ink wash drifting slowly, Bích Lam a jade-azure ripple,
     Kim Sa gold with drifting gold-sand glints, Tinh Hà a violet-cyan river with tiny stars, Linh Quang / Tử Quang a
     soft halo. The flow runs at effects full / lite; off / reduced motion = the static gradient. -->
<script lang="ts">
	import type { Snippet } from 'svelte';
	import { nameFxClass } from '$lib/game/cosmetics.svelte';

	interface Props {
		fx?: string;
		class?: string;
		children: Snippet;
	}
	let { fx, class: cls = '', children }: Props = $props();
</script>

<span class={['namefx', nameFxClass(fx), cls]}>{@render children()}</span>

<style>
	.namefx[class*='fx-'] {
		background-size: 200% 100%;
		-webkit-background-clip: text;
		background-clip: text;
		color: transparent;
	}
	:global(:root[data-fx='full']) .namefx[class*='fx-'],
	:global(:root[data-fx='lite']) .namefx[class*='fx-'] {
		animation: fx-flow 6s linear infinite;
	}
	.fx-mac-huong {
		background-image: linear-gradient(90deg, var(--color-mist-100), var(--color-mist-500), var(--color-mist-100), var(--color-mist-300));
		filter: drop-shadow(0 0 3px color-mix(in oklab, var(--color-ink-600) 90%, transparent));
	}
	:global(:root[data-fx='full']) .fx-mac-huong,
	:global(:root[data-fx='lite']) .fx-mac-huong {
		animation-duration: 11s;
	}
	.fx-bich-lam {
		background-image: linear-gradient(90deg, var(--color-jade-300), var(--color-spirit-cyan), var(--color-jade-300), var(--color-aura-azure));
		filter: drop-shadow(0 0 3px color-mix(in oklab, var(--color-spirit-cyan) 45%, transparent));
	}
	.fx-kim-sa {
		background-image:
			radial-gradient(circle, var(--color-mist-100) 0 0.6px, transparent 1.2px),
			linear-gradient(90deg, var(--color-gold-500), var(--color-gold-300), var(--color-aura-radiant), var(--color-gold-500));
		background-size:
			7px 6px,
			200% 100%;
		filter: drop-shadow(0 0 4px color-mix(in oklab, var(--color-gold-300) 45%, transparent));
	}
	.fx-tinh-ha {
		background-image:
			radial-gradient(circle, var(--color-mist-100) 0 0.7px, transparent 1.3px),
			linear-gradient(90deg, var(--color-spirit-violet), var(--color-spirit-cyan), var(--color-aura-celestial), var(--color-spirit-violet));
		background-size:
			11px 9px,
			200% 100%;
		filter: drop-shadow(0 0 5px color-mix(in oklab, var(--color-spirit-violet) 55%, transparent));
	}
	.fx-linh-quang {
		background-image: linear-gradient(90deg, var(--color-jade-300), var(--color-aura-celestial), var(--color-jade-300));
		filter: drop-shadow(0 0 4px color-mix(in oklab, var(--color-jade-300) 60%, transparent)) drop-shadow(0 0 10px color-mix(in oklab, var(--color-jade-300) 30%, transparent));
	}
	.fx-tu-quang {
		background-image: linear-gradient(90deg, var(--color-spirit-violet), var(--color-aura-primal), var(--color-spirit-violet));
		filter: drop-shadow(0 0 4px color-mix(in oklab, var(--color-spirit-violet) 60%, transparent)) drop-shadow(0 0 10px color-mix(in oklab, var(--color-spirit-violet) 30%, transparent));
	}
	@keyframes fx-flow {
		to {
			background-position:
				-40px 12px,
				-200% 0;
		}
	}
</style>
