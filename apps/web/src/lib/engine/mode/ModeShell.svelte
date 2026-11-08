<!--
	Frame of a full-screen mini-game MODE (spec §3.3, §4.9). Rendered by a mini-game route; the game shell puts mode
	routes in the ModeLayer and hides the HUD, action bar, chat and windows (the scene behind stays mounted, paused).
	It owns: the minimal mode HUD (title, pause, exit), the 3-2-1 countdown ("Bắt đầu" at once when effects are off),
	the exit confirmation, Esc and browser Back (pause first, then confirm — machine.ts), the "waiting for Thiên đạo"
	gather loop and the soft error with retry, and an orientation hint on phones. The route renders the lobby, the
	board and the result as children and reacts to the controller's phase.
-->
<script lang="ts">
	import type { Snippet } from 'svelte';
	import { beforeNavigate } from '$app/navigation';
	import Icon, { type IconName } from '$lib/components/Icon.svelte';
	import { fxBus } from '../fx/bus';
	import { settings } from '../settings.svelte';
	import { gatherQi } from './gather';
	import { countdownLabel } from './machine';
	import type { ModeController } from './mode.svelte';

	interface Props {
		mode: ModeController;
		title: string;
		subtitle?: string;
		icon: IconName;
		/** a portrait board: on a phone held sideways, suggest turning it upright */
		orientation?: 'portrait' | 'landscape' | 'any';
		/** shown while the server judges */
		waitingText: string;
		/** "Rời đi?" text during play (what giving up means for this game) */
		confirmText: string;
		onretry: () => void;
		children: Snippet;
	}
	let { mode, title, subtitle, icon, orientation = 'any', waitingText, confirmText, onretry, children }: Props = $props();

	const s = $derived(mode.state);
	const label = $derived(countdownLabel(s));
	const inGame = $derived(s.phase === 'countdown' || s.phase === 'play');
	// Sound (fxBus `countdown`): a tick per 3-2-1 number, a brighter one on "Bắt đầu".
	$effect(() => {
		if (label) fxBus.emit('countdown', { n: label === 'Bắt đầu' ? 0 : Number(label) });
	});

	// Esc = Back (pause → confirm). Inputs keep their Esc.
	function onkeydown(e: KeyboardEvent) {
		if (e.key !== 'Escape' || e.repeat) return;
		const t = e.target as HTMLElement | null;
		if (t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.isContentEditable)) return;
		e.preventDefault();
		mode.send({ type: 'back' });
	}
	// Browser/phone Back while a run is live (or its result is unsent): stay, and treat it as Esc.
	beforeNavigate((nav) => {
		if (nav.type !== 'popstate' || nav.willUnload) return;
		const p = mode.state.phase;
		if (p === 'lobby' || p === 'result') return; // leaving is fine
		nav.cancel();
		mode.send({ type: 'back' });
	});
	const exitClick = () => {
		if (inGame && !s.paused) mode.send({ type: 'pause' });
		mode.send({ type: 'back' });
	};

	let seal = $state<HTMLElement>();
	$effect(() => {
		if (s.phase !== 'waiting' || !seal) return;
		const el = seal;
		return gatherQi(() => {
			const r = el.getBoundingClientRect();
			return r.width ? { x: r.left + r.width / 2, y: r.top + r.height / 2 } : null;
		}, settings.fx);
	});

	let confirmBtn = $state<HTMLButtonElement>();
	$effect(() => {
		if (s.confirm) confirmBtn?.focus();
	});
</script>

<svelte:window {onkeydown} />

<div class="mode" class:is-portrait={orientation === 'portrait'} data-mode={s.phase} data-mode-paused={s.paused || undefined}>
	<header class="mode-hud">
		<span class="mark"><Icon name={icon} size={18} /></span>
		<div class="min-w-0 flex-1">
			<h1 class="truncate font-display text-lg leading-tight font-bold text-gold-300 sm:text-xl">{title}</h1>
			{#if subtitle}<p class="truncate text-xs text-mist-500">{subtitle}</p>{/if}
		</div>
		{#if inGame}
			<button
				type="button"
				class="btn btn-ghost h-10 px-3 text-sm"
				onclick={() => mode.send({ type: s.paused ? 'resume' : 'pause' })}
				aria-pressed={s.paused}
				data-mode-pause
			>
				<Icon name={s.paused ? 'lotus' : 'clock'} size={16} />
				<span class="hidden sm:inline">{s.paused ? 'Tiếp tục' : 'Tạm dừng'}</span>
				<span class="sr-only sm:hidden">{s.paused ? 'Tiếp tục' : 'Tạm dừng'}</span>
			</button>
		{/if}
		{#if s.phase !== 'waiting'}
			<button type="button" class="btn btn-ghost size-10 p-0" onclick={exitClick} aria-label={inGame ? 'Thoát thử luyện' : 'Về'} data-mode-exit>
				<Icon name={inGame || s.phase === 'error' ? 'exit' : 'cross'} size={18} />
			</button>
		{/if}
	</header>

	<div class="mode-body">
		{@render children()}

		{#if s.phase === 'waiting' || s.phase === 'error'}
			<div class="judge" data-mode-judge={s.phase}>
				<div class="text-center">
					<div bind:this={seal} class="seal mx-auto grid size-24 place-items-center rounded-full text-gold-300" class:is-breathe={s.phase === 'waiting'}>
						<Icon name={icon} size={36} />
					</div>
					{#if s.phase === 'error'}
						<p class="mt-5 text-mist-100" role="alert">Thiên đạo chưa hồi âm.</p>
						<p class="mt-1 text-sm text-mist-500">Ván chơi được giữ trên trang này. Gửi lại trước khi phiên hết hạn — gửi lại không tính thêm lượt.</p>
						<button type="button" class="btn btn-jade mt-4" onclick={onretry} data-mode-retry>Gửi lại</button>
					{:else}
						<p class="mt-5 font-display text-2xl text-gold-300" aria-live="polite">{waitingText}</p>
						<p class="mt-1 text-sm text-mist-500">Kết quả chỉ do máy chủ phán định.</p>
					{/if}
				</div>
			</div>
		{/if}

		{#if label}
			{#key label}
				<div class="count" class:is-go={label === 'Bắt đầu'} aria-live="assertive" data-mode-countdown>
					<span class="font-display">{label}</span>
				</div>
			{/key}
		{/if}
	</div>

	{#if orientation === 'portrait'}
		<p class="turn-hint" aria-hidden="true"><Icon name="arrow" size={16} /> Xoay dọc điện thoại để chơi thoải mái hơn</p>
	{/if}

	{#if s.confirm}
		<div class="scrim" role="presentation">
			<div class="confirm panel" role="alertdialog" aria-modal="true" aria-labelledby="mode-confirm-t" data-mode-confirm>
				<p id="mode-confirm-t" class="font-display text-2xl font-bold text-mist-100">{inGame ? 'Rời thử luyện?' : 'Rời đi?'}</p>
				<p class="mt-2 text-sm text-mist-300">{inGame ? confirmText : 'Kết quả chưa gửi sẽ không được ghi nhận.'}</p>
				<div class="mt-4 grid grid-cols-2 gap-2">
					<button bind:this={confirmBtn} type="button" class="btn btn-jade" onclick={() => mode.send({ type: 'cancelExit' })}>Ở lại</button>
					<button type="button" class="btn btn-ghost" onclick={() => mode.send({ type: 'confirmExit' })} data-mode-confirm-exit>Rời đi</button>
				</div>
			</div>
		</div>
	{/if}
</div>

<style>
	.mode {
		position: fixed;
		inset: 0;
		display: flex;
		flex-direction: column;
		pointer-events: auto;
		background:
			radial-gradient(ellipse at 50% 0%, color-mix(in oklab, var(--color-ink-700) 40%, transparent), transparent 60%),
			var(--color-ink-950);
		animation: mode-in var(--dur-base) var(--ease-qi);
		overscroll-behavior: none;
	}
	@keyframes mode-in {
		from {
			opacity: 0;
			transform: scale(1.02);
		}
	}
	.mode-hud {
		display: flex;
		flex-shrink: 0;
		align-items: center;
		gap: 0.6rem;
		padding: 0.5rem 0.75rem;
		padding-top: max(0.5rem, env(safe-area-inset-top));
		border-bottom: 1px solid color-mix(in oklab, var(--color-gold-500) 18%, transparent);
		background: color-mix(in oklab, var(--color-ink-900) 85%, transparent);
	}
	.mark {
		display: grid;
		place-items: center;
		width: 2.25rem;
		height: 2.25rem;
		flex-shrink: 0;
		border-radius: 9999px;
		color: var(--color-gold-300);
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 45%, transparent);
	}
	.mode-body {
		position: relative;
		flex: 1;
		min-height: 0;
	}
	.judge {
		position: absolute;
		inset: 0;
		display: grid;
		place-items: center;
		padding: 1.5rem;
		background: var(--color-ink-950);
	}
	.seal {
		background: radial-gradient(circle, color-mix(in oklab, var(--color-gold-500) 14%, var(--color-ink-800)), var(--color-ink-950));
		box-shadow:
			0 0 0 1px var(--color-gold-500),
			0 0 0 6px var(--color-ink-950),
			0 0 0 7px color-mix(in oklab, var(--color-gold-500) 40%, transparent);
	}
	:global(:root[data-fx='full']) .seal.is-breathe,
	:global(:root[data-fx='lite']) .seal.is-breathe {
		animation: var(--animate-qi-breathe);
	}
	.count {
		position: absolute;
		inset: 0;
		display: grid;
		place-items: center;
		pointer-events: none;
		background: color-mix(in oklab, var(--color-ink-950) 35%, transparent);
	}
	.count span {
		font-size: clamp(4rem, 18vw, 8rem);
		font-weight: 700;
		line-height: 1;
		color: var(--color-gold-300);
		text-shadow: 0 0 30px color-mix(in oklab, var(--color-gold-300) 45%, transparent);
	}
	.count.is-go {
		background: none;
	}
	.count.is-go span {
		font-size: clamp(2.5rem, 10vw, 4.5rem);
		color: var(--color-jade-300);
	}
	:global(:root[data-fx='full']) .count span {
		animation: count-pop 900ms var(--ease-out-back);
	}
	:global(:root[data-fx='lite']) .count span {
		animation: count-fade var(--dur-base) var(--ease-qi);
	}
	@keyframes count-pop {
		from {
			opacity: 0;
			transform: scale(1.6);
		}
		30% {
			opacity: 1;
			transform: scale(1);
		}
	}
	@keyframes count-fade {
		from {
			opacity: 0;
		}
	}
	.turn-hint {
		display: none;
	}
	/* phones held sideways with a portrait board */
	@media (orientation: landscape) and (max-height: 500px) and (pointer: coarse) {
		.turn-hint {
			position: absolute;
			left: 50%;
			bottom: 0.75rem;
			transform: translateX(-50%);
			display: flex;
			align-items: center;
			gap: 0.4rem;
			padding: 0.35rem 0.8rem;
			border-radius: 9999px;
			font-size: 0.8rem;
			color: var(--color-mist-100);
			background: color-mix(in oklab, var(--color-ink-900) 90%, transparent);
			border: 1px solid color-mix(in oklab, var(--color-gold-500) 35%, transparent);
			pointer-events: none;
		}
		.turn-hint :global(svg) {
			transform: rotate(-90deg);
		}
	}
	.scrim {
		position: absolute;
		inset: 0;
		display: grid;
		place-items: center;
		padding: 1rem;
		background: color-mix(in oklab, var(--color-ink-950) 70%, transparent);
	}
	.confirm {
		width: min(100%, 24rem);
		padding: 1.25rem;
		text-align: center;
		animation: confirm-in var(--dur-window) var(--ease-qi);
	}
	@keyframes confirm-in {
		from {
			opacity: 0;
			transform: scale(0.96);
		}
	}
</style>
