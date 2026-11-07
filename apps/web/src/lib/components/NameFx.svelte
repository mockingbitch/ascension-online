<!-- A character name (or any text) with its chat name effect (cosmetics.yaml chat_aura:*). Looks only; the gradient/
     glow is decorative, the text keeps its own colour underneath for contrast. Motion stops under reduced motion. -->
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
		animation: fx-flow 6s linear infinite;
	}
	.fx-mac-huong {
		background-image: linear-gradient(90deg, var(--color-mist-100), var(--color-mist-500), var(--color-mist-100), var(--color-mist-300));
	}
	.fx-bich-lam {
		background-image: linear-gradient(90deg, var(--color-jade-300), var(--color-spirit-cyan), var(--color-jade-300), var(--color-aura-azure));
	}
	.fx-kim-sa {
		background-image: linear-gradient(90deg, var(--color-gold-500), var(--color-gold-300), var(--color-aura-radiant), var(--color-gold-500));
	}
	.fx-tinh-ha {
		background-image: linear-gradient(90deg, var(--color-spirit-violet), var(--color-spirit-cyan), var(--color-aura-celestial), var(--color-spirit-violet));
	}
	.fx-linh-quang {
		background-image: linear-gradient(90deg, var(--color-jade-300), var(--color-aura-celestial), var(--color-jade-300));
		filter: drop-shadow(0 0 4px color-mix(in oklab, var(--color-jade-300) 60%, transparent));
	}
	.fx-tu-quang {
		background-image: linear-gradient(90deg, var(--color-spirit-violet), var(--color-aura-primal), var(--color-spirit-violet));
		filter: drop-shadow(0 0 4px color-mix(in oklab, var(--color-spirit-violet) 60%, transparent));
	}
	@keyframes fx-flow {
		to {
			background-position: -200% 0;
		}
	}
</style>
