<!-- Toast stack at the top-centre of the game screen. -->
<script lang="ts">
	import { dismiss, toasts } from '$lib/game/toasts.svelte';
	import Icon from './Icon.svelte';
</script>

<div class="pointer-events-none fixed inset-x-0 top-16 z-50 flex flex-col items-center gap-2 px-3" role="status" aria-live="polite">
	{#each toasts.items as t (t.id)}
		<button
			type="button"
			class="toast pointer-events-auto flex max-w-md items-start gap-2 px-4 py-2.5 text-left text-sm"
			class:tone-ok={t.tone === 'ok'}
			class:tone-warn={t.tone === 'warn'}
			class:tone-err={t.tone === 'err'}
			onclick={() => dismiss(t.id)}
		>
			<Icon name={t.tone === 'ok' ? 'spark' : t.tone === 'warn' ? 'clock' : 'cross'} size={18} class="mt-px shrink-0" />
			<span class="text-mist-100">{t.text}</span>
		</button>
	{/each}
</div>

<style>
	.toast {
		border: 1px solid color-mix(in oklab, currentColor 45%, transparent);
		border-left-width: 3px;
		background: color-mix(in oklab, var(--color-ink-950) 88%, transparent);
		backdrop-filter: blur(6px);
		box-shadow: 0 10px 30px -12px var(--color-ink-950);
		animation: toast-in 260ms var(--ease-qi);
	}
	.tone-ok {
		color: var(--color-jade-300);
	}
	.tone-warn {
		color: var(--color-gold-300);
	}
	.tone-err {
		color: var(--color-cinnabar-300);
	}
	@keyframes toast-in {
		from {
			opacity: 0;
			transform: translateY(-8px);
		}
	}
</style>
