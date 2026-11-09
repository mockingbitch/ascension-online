<!--
	Động Phủ · Tu Luyện: the idle-cultivation core. The cave, the cultivator and the aura are the dong_phu SCENE
	(SceneLayer + DongPhuView); this page is UI over it — the slot the scene fills, the qi bar, a few big numbers and
	the Đột phá button. "Thu nhận" is the action bar's primary button (registered here, runs the store's settle). Rules
	live behind "?" (help window), the daily board in the Nhiệm vụ window, the fortune in Gieo quẻ. All values are server
	state; the bar/clock interpolate for display.
-->
<script lang="ts">
	import HelpButton from '$lib/components/HelpButton.svelte';
	import Icon from '$lib/components/Icon.svelte';
	import QiBar from '$lib/components/QiBar.svelte';
	import RealmBadge from '$lib/components/RealmBadge.svelte';
	import SceneSlot from '$lib/engine/scene/SceneSlot.svelte';
	import { formatBp, formatPoints } from '$lib/game/format';
	import { useIdleClock } from '$lib/game/idle.svelte';
	import { usePrimaryAction } from '$lib/game/primary.svelte';
	import { game } from '$lib/game/store.svelte';
	import { openWindow, prefetchWindow } from '$lib/game/windows';
	import { MediaQuery } from 'svelte/reactivity';

	const clock = useIdleClock();
	const phone = new MediaQuery('max-width: 639px');
	const ch = $derived(game.character!);
	const cat = $derived(game.catalogue!);
	const stage = $derived(cat.stages.get(ch.stageId));
	const realm = $derived(cat.realmOf(ch.stageId));
	const aura = $derived(realm?.visual.aura ?? 'jade');
	const subStage = $derived(stage && realm && stage.name.startsWith(realm.name) ? stage.name.slice(realm.name.length).trim() : (stage?.name ?? ''));
	const pv = $derived(game.preview);
	const phu = $derived(cat.items.get('tu_linh_phu'));
	const capped = $derived(ch.today.pointsMilli >= ch.today.ceilingMilli);

	// One headline state, most important first.
	const state = $derived.by(() => {
		if (pv?.ready) return { tone: 'gold', icon: 'bolt', text: 'Đủ điều kiện đột phá' } as const;
		if (clock.atBottleneck) return { tone: 'gold', icon: 'lock', text: 'Bình cảnh — cần đột phá' } as const;
		if (clock.isFull) return { tone: 'gold', icon: 'clock', text: 'Linh khí đã đầy — hãy thu nhận' } as const;
		if (capped) return { tone: 'mist', icon: 'check', text: 'Đã đạt hạn mức hôm nay' } as const;
		return { tone: 'jade', icon: 'lotus', text: 'Đang tu luyện' } as const;
	});

	usePrimaryAction(() => ({
		id: 'settle',
		label: 'Thu nhận',
		icon: 'lotus',
		hint: clock.isFull ? 'Đã đầy' : clock.elapsedLabel,
		ready: clock.shown > ch.cultivationMilli,
		disabled: game.busy,
		busy: game.busy,
		progress: clock.capSeconds ? clock.elapsedSeconds / clock.capSeconds : 0,
		tone: 'jade',
		run: game.settle
	}));
</script>

<svelte:head><title>Tu Luyện · Tiên Đạo</title></svelte:head>

<!-- one scroller (the stage); on phones: one-line title → scene (flexes, min ~38% of the stage) → qi card, with the
     Đột phá / Gieo quẻ row sticky at the stage bottom, so it is always on screen (UI polish U2, A1) -->
<div class="dong-phu" data-stage-scroll>
	<header class="head ui" data-toast-below>
		<p class="eyebrow hidden sm:block">Động Phủ · bế quan</p>
		<span class="chip" data-tone={state.tone} data-state-chip><Icon name={state.icon} size={14} /> <span class="truncate">{state.text}</span></span>
		<div class="title">
			{#if realm}<RealmBadge name={realm.name} aura={realm.visual.aura} size={phone.current ? 'sm' : 'lg'} />{/if}
			<span class="font-display text-mist-100 sm:text-lg">{subStage}</span>
		</div>
		<HelpButton topic="dong_phu" class="help-pos" />
	</header>

	<SceneSlot class="scene-box" />

	<div class="ui controls panel">
		<div class="px-3 pt-3" data-fx-target="qi" data-tutorial="qi">
			<QiBar current={clock.shown} required={ch.requiredMilli} cap={ch.capMilli} {aura} label="Tu vi · ước tính" />
		</div>
		<dl class="stats stagger mt-3 grid grid-cols-3 gap-2 px-3 text-center">
			<div>
				<dt><Icon name="spark" size={14} /> Tốc độ</dt>
				<dd class="text-jade-300">+{formatPoints(ch.idleRateMilliPerHour)}<small>/giờ</small></dd>
			</div>
			<div>
				<dt><Icon name="clock" size={14} /> Bế quan</dt>
				<dd class:text-gold-300={clock.isFull}>{clock.elapsedLabel}</dd>
				<p class="sub">{clock.isFull ? 'Đã đầy' : `đầy sau ${clock.untilFullLabel}`} · tối đa {Math.round(clock.capSeconds / 3600)} giờ</p>
			</div>
			<div>
				<dt><Icon name="check" size={14} /> Hôm nay</dt>
				<dd class:text-gold-300={capped}>{formatPoints(ch.today.pointsMilli)}</dd>
				<p class="sub">/ {formatPoints(ch.today.ceilingMilli)}{capped ? ' · đã đủ' : ''}</p>
			</div>
		</dl>
		{#if ch.today.catchUp.active || ch.offlineBonusSeconds > 0}
			<p class="mt-2 flex flex-wrap justify-center gap-2 px-3 text-xs">
				{#if ch.today.catchUp.active}<span class="tag text-gold-300"><Icon name="bolt" size={12} /> Bù đắp ×{ch.today.catchUp.multiplierBp / 10000}</span>{/if}
				{#if ch.offlineBonusSeconds > 0}<span class="tag text-jade-300"><Icon name="talisman" size={12} /> +{ch.offlineBonusSeconds / 3600} giờ {phu?.name ?? 'Tụ Linh Phù'}</span>{/if}
			</p>
		{/if}
		<div class="actions" data-region="actions">
			<button type="button" class="btn btn-gold h-12 flex-1 px-2 text-sm whitespace-nowrap sm:text-base" class:is-ready={pv?.ready} onclick={() => openWindow({ kind: 'breakthrough' })} onpointerenter={() => prefetchWindow('breakthrough')} onpointerdown={() => prefetchWindow('breakthrough')} onfocus={() => prefetchWindow('breakthrough')} data-open-breakthrough>
				<Icon name="bolt" size={18} stroke={1.8} /> Đột phá{#if pv && !pv.missing.some((m) => m.kind === 'final_stage')}<span class="tabular-nums opacity-80">{formatBp(pv.successBp)}</span>{/if}
			</button>
			{#if ch.offlineBonusSeconds <= 0 && game.phuCount > 0 && !ch.bonusUsedToday}
				<button class="btn btn-ghost h-12 px-3" disabled={game.busy} aria-busy={game.busy || undefined} onclick={game.usePhu} title="Dùng {phu?.name ?? 'Tụ Linh Phù'}{phu?.offlineCapBonusHours ? ` (+${phu.offlineCapBonusHours} canh giờ)` : ''}">
					<Icon name="talisman" size={18} /><span class="sr-only sm:not-sr-only">{phu?.name ?? 'Tụ Linh Phù'}</span>
				</button>
			{/if}
			<button type="button" class="btn btn-ghost h-12 px-3 whitespace-nowrap" onclick={() => openWindow({ kind: 'fortune' })} onpointerenter={() => prefetchWindow('fortune')} onpointerdown={() => prefetchWindow('fortune')} onfocus={() => prefetchWindow('fortune')}>
				<Icon name="dice" size={18} /><span class="sr-only sm:not-sr-only">Gieo quẻ</span>
			</button>
		</div>
	</div>
</div>

<style>
	/* the stage's one scroller (overscroll stays inside: no scene/page chaining) */
	.dong-phu {
		display: flex;
		height: 100%;
		min-height: 0;
		flex-direction: column;
		overflow-y: auto;
		overscroll-behavior: contain;
		padding: 0.25rem 0.25rem 0;
	}
	.head {
		position: relative;
		display: flex;
		flex-shrink: 0;
		flex-direction: column;
		align-items: center;
		gap: 0.25rem;
		text-align: center;
	}
	.title {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 0.25rem;
		margin-top: 0.25rem;
	}
	.head :global(.help-pos) {
		position: absolute;
		top: 0;
		right: 0;
	}
	/* the scene fills what is left: on phones never less than ~38% of the stage */
	.dong-phu :global(.scene-box) {
		flex: 1 0 auto;
		width: 100%;
		max-width: 760px;
		min-height: max(160px, 38%);
		margin-inline: auto;
	}
	.controls {
		flex-shrink: 0;
		width: 100%;
		max-width: 36rem;
		margin-inline: auto;
		backdrop-filter: blur(4px);
	}
	/* the action row sticks to the stage bottom (always reachable, never under the bar / chat dock) */
	.actions {
		position: sticky;
		bottom: 0;
		z-index: var(--z-local-1);
		display: flex;
		gap: var(--tap-gap);
		margin-top: 0.75rem;
		padding: 0.5rem 0.75rem 0.75rem;
		background: linear-gradient(180deg, color-mix(in oklab, var(--color-ink-900) 70%, transparent), var(--color-ink-900) 40%);
	}
	/* phones: one-line title (badge · tầng · state chip · ?) */
	@media (max-width: 639px) {
		.head {
			flex-direction: row;
			flex-wrap: nowrap;
			justify-content: flex-start;
			gap: 0.5rem;
			padding-right: calc(var(--tap) + 0.25rem);
			min-height: var(--tap);
			text-align: left;
		}
		.title {
			order: -1;
			flex-direction: row;
			align-items: center;
			gap: 0.4rem;
			margin: 0;
			flex-shrink: 0;
		}
		.chip {
			min-width: 0;
			overflow: hidden;
		}
		.dong-phu :global(.scene-box) {
			flex: 1 1 auto;
			margin-block: 0.25rem;
		}
	}
	/* wide: the chat dock owns the stage's bottom-left corner; the card stays right of it when the stage is narrow */
	@media (min-width: 640px) {
		.controls {
			margin-left: max(calc(var(--chat-w, 0px) + 0.75rem), calc((100% - 36rem) / 2));
			margin-right: max(0px, calc((100% - 36rem) / 2));
			width: auto;
			margin-bottom: 0.25rem;
		}
	}
	/* landscape phones (short + wide): scene left, title + card right, the right column scrolls */
	@media (min-width: 640px) and (max-height: 560px) {
		.dong-phu {
			display: grid;
			grid-template-columns: minmax(0, 1fr) minmax(16rem, 22rem);
			grid-template-rows: auto minmax(0, 1fr);
			grid-template-areas: 'scene head' 'scene card';
			column-gap: 0.75rem;
		}
		.head {
			grid-area: head;
		}
		.dong-phu :global(.scene-box) {
			grid-area: scene;
			min-height: 0;
			height: 100%;
			position: sticky;
			top: 0;
		}
		.controls {
			grid-area: card;
			align-self: end;
			margin: 0;
			width: 100%;
		}
		.stats :global(.sub) {
			display: none;
		}
	}
	.chip {
		display: inline-flex;
		align-items: center;
		gap: 0.35rem;
		padding: 2px 10px;
		font-size: 0.75rem;
		font-weight: 600;
		border: 1px solid currentColor;
		border-radius: 9999px;
		background: color-mix(in oklab, var(--color-ink-950) 60%, transparent);
	}
	.chip[data-tone='jade'] {
		color: var(--color-jade-300);
	}
	.chip[data-tone='gold'] {
		color: var(--color-gold-300);
	}
	.chip[data-tone='mist'] {
		color: var(--color-mist-300);
	}
	.stats > div {
		padding: 0.45rem 0.25rem;
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 15%, transparent);
		background: color-mix(in oklab, var(--color-ink-950) 45%, transparent);
	}
	.stats dt {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 0.25rem;
		font-size: 0.7rem;
		color: var(--color-mist-300);
	}
	.stats dd {
		margin-top: 1px;
		font-size: 1.15rem;
		font-weight: 700;
		font-variant-numeric: tabular-nums;
		color: var(--color-mist-100);
	}
	.stats small {
		font-size: 0.65rem;
		font-weight: 500;
		color: var(--color-mist-300);
	}
	.sub {
		font-size: 0.65rem;
		line-height: 1.2;
		color: var(--color-mist-300);
		font-variant-numeric: tabular-nums;
	}
	.tag {
		display: inline-flex;
		align-items: center;
		gap: 0.25rem;
		padding: 1px 8px;
		border: 1px solid currentColor;
		border-radius: 9999px;
	}
	/* ready: a glow layer breathes (opacity only — no box-shadow animation) */
	.is-ready {
		position: relative;
	}
	.is-ready::after {
		content: '';
		position: absolute;
		inset: 0;
		border-radius: inherit;
		box-shadow: 0 0 22px 2px color-mix(in oklab, var(--color-gold-300) 45%, transparent);
		opacity: 0.5;
		pointer-events: none;
	}
	:global(:root[data-fx='full']) .is-ready::after,
	:global(:root[data-fx='lite']) .is-ready::after {
		animation: ready-glow 2.4s var(--ease-in-out-sine) infinite;
	}
	@keyframes ready-glow {
		0%,
		100% {
			opacity: 0;
		}
		50% {
			opacity: 1;
		}
	}
</style>
