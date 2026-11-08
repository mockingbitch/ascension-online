<!--
	"Thêm vào màn hình chính" hint on phones (spec §4.11, engine/install.svelte.ts): a small dismissible card just under
	the HUD (over the scene, clear of the primary action, chat and action bar). Chromium: "Cài" opens the native install
	prompt (beforeinstallprompt). iOS Safari (no such event): "Xem cách" opens short instructions in a local GameWindow.
	"Để sau" / ✕ hides it for good (localStorage).
-->
<script lang="ts">
	import { installHint } from '$lib/engine/install.svelte';
	import GameWindow from './GameWindow.svelte';
	import Icon from './Icon.svelte';

	let howOpen = $state(false);
	const mode = $derived(installHint.mode);
</script>

{#if mode}
	<aside class="hint panel" aria-label="Thêm vào màn hình chính" data-install-hint={mode}>
		<span class="mark" aria-hidden="true"><Icon name="home" size={20} /></span>
		<p class="min-w-0 flex-1 text-sm leading-snug text-mist-100">Thêm vào màn hình chính để chơi toàn màn hình.</p>
		{#if mode === 'prompt'}
			<button type="button" class="btn btn-gold h-10 px-3 text-sm" onclick={() => installHint.prompt()} data-install-go>Cài</button>
		{:else}
			<button type="button" class="btn btn-gold h-10 px-3 text-sm" onclick={() => (howOpen = true)} data-install-go>Xem cách</button>
		{/if}
		<button type="button" class="btn btn-ghost size-10 p-0" aria-label="Để sau" onclick={() => installHint.dismiss()} data-install-dismiss>
			<Icon name="cross" size={16} />
		</button>
	</aside>
{/if}

<GameWindow open={howOpen} local title="Thêm vào màn hình chính" icon="home" onclose={() => (howOpen = false)}>
	<ol class="steps space-y-3 text-sm text-mist-100" data-install-ios>
		<li><span class="n">1</span> Mở trang này bằng <b>Safari</b>.</li>
		<li><span class="n">2</span> Chạm nút <b>Chia sẻ</b> <Icon name="share" size={16} class="inline align-text-bottom text-gold-300" /> ở thanh dưới.</li>
		<li><span class="n">3</span> Chọn <b>Thêm vào MH chính</b>, rồi <b>Thêm</b>.</li>
		<li><span class="n">4</span> Mở Tiên Đạo từ biểu tượng mới: trò chơi chạy toàn màn hình.</li>
	</ol>
	<div class="mt-5 flex justify-end gap-2">
		<button
			type="button"
			class="btn btn-ghost h-10 px-3 text-sm"
			onclick={() => {
				howOpen = false;
				installHint.dismiss();
			}}>Không nhắc nữa</button
		>
		<button type="button" class="btn btn-jade h-10 px-4 text-sm" onclick={() => (howOpen = false)}>Đã hiểu</button>
	</div>
</GameWindow>

<style>
	.hint {
		position: fixed;
		left: 0.75rem;
		right: 0.75rem;
		top: calc(var(--hud-h, 3.5rem) + 0.5rem + env(safe-area-inset-top));
		z-index: var(--z-toast);
		display: flex;
		align-items: center;
		gap: 0.5rem;
		max-width: 28rem;
		margin-inline: auto;
		padding: 0.5rem 0.5rem 0.5rem 0.75rem;
		animation: hint-in var(--dur-slow) var(--ease-qi) both;
	}
	.mark {
		display: grid;
		width: 2.25rem;
		height: 2.25rem;
		flex-shrink: 0;
		place-items: center;
		border-radius: 9999px;
		color: var(--color-gold-300);
		background: color-mix(in oklab, var(--color-gold-500) 14%, transparent);
	}
	@keyframes hint-in {
		from {
			opacity: 0;
			transform: translateY(-8px);
		}
	}
	.steps li {
		display: flex;
		align-items: baseline;
		gap: 0.6rem;
	}
	.n {
		display: inline-grid;
		width: 1.5rem;
		height: 1.5rem;
		flex-shrink: 0;
		place-items: center;
		border-radius: 9999px;
		font-weight: 700;
		color: var(--color-ink-950);
		background: var(--color-gold-300);
	}
</style>
