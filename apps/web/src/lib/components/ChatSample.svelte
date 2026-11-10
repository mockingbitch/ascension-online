<!--
	A sample chat line with a chat name effect (cosmetics.yaml chat_aura:*), as others see it in Kênh chat: the name in
	its effect and a thin border in the effect's colours flowing round the bubble (effects full / lite). Preview only.
-->
<script lang="ts">
	import { nameFxClass } from '$lib/game/cosmetics.svelte';
	import NameFx from './NameFx.svelte';

	let { name, fx, text = 'Đạo hữu an hảo.' }: { name: string; fx?: string; text?: string } = $props();
</script>

<p class="bubble {fx ? `b-${nameFxClass(fx)}` : ''}" data-chat-sample={fx ?? ''}>
	<NameFx {fx} class="font-semibold">{name}</NameFx>
	<span class="text-mist-300">: {text}</span>
</p>

<style>
	.bubble {
		--grad: linear-gradient(90deg, color-mix(in oklab, var(--color-gold-500) 30%, transparent), color-mix(in oklab, var(--color-gold-500) 30%, transparent));
		display: block;
		max-width: 100%;
		padding: 0.4rem 0.7rem;
		font-size: 0.85rem;
		line-height: 1.4;
		border: 1.5px solid transparent;
		border-radius: 0.5rem;
		background:
			linear-gradient(var(--color-ink-950), var(--color-ink-950)) padding-box,
			var(--grad) border-box;
		background-size:
			100% 100%,
			300% 100%;
		overflow: hidden;
		white-space: nowrap;
		text-overflow: ellipsis;
	}
	:global(:root[data-fx='full']) .bubble,
	:global(:root[data-fx='lite']) .bubble {
		animation: border-flow 7s linear infinite;
	}
	@keyframes border-flow {
		to {
			background-position:
				0 0,
				-300% 0;
		}
	}
	.b-fx-mac-huong {
		--grad: linear-gradient(90deg, var(--color-mist-500), var(--color-ink-600), var(--color-mist-300), var(--color-mist-500));
	}
	.b-fx-bich-lam {
		--grad: linear-gradient(90deg, var(--color-jade-300), var(--color-spirit-cyan), var(--color-aura-azure), var(--color-jade-300));
	}
	.b-fx-kim-sa {
		--grad: linear-gradient(90deg, var(--color-gold-700), var(--color-gold-300), var(--color-aura-radiant), var(--color-gold-700));
	}
	.b-fx-tinh-ha {
		--grad: linear-gradient(90deg, var(--color-spirit-violet), var(--color-spirit-cyan), var(--color-aura-celestial), var(--color-spirit-violet));
	}
	.b-fx-linh-quang {
		--grad: linear-gradient(90deg, var(--color-jade-300), var(--color-aura-celestial), var(--color-jade-300));
		box-shadow: 0 0 10px color-mix(in oklab, var(--color-jade-300) 30%, transparent);
	}
	.b-fx-tu-quang {
		--grad: linear-gradient(90deg, var(--color-spirit-violet), var(--color-aura-primal), var(--color-spirit-violet));
		box-shadow: 0 0 10px color-mix(in oklab, var(--color-spirit-violet) 35%, transparent);
	}
</style>
