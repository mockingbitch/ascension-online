<!--
	Breakthrough / lôi kiếp MODE (spec §4.6), lazy. Full screen over the scene (the shell hides HUD, bar, chat and the
	Đột Phá window). Three states from btMode:
	  gather   the request is in flight: aura + formation + converging motes loop (no outcome is known yet);
	  stall    no answer in time / network: soft error, "Thử lại" joins the same request (same Idempotency-Key);
	  outcome  the server answered: play the planned beats (fx/breakthrough.ts, ≤ 3 s), skippable at any moment
	           (tap / Esc / Back / "Bỏ qua"), then the static result card with the server's numbers and text.
	Effects off = the static card at once; reduced motion = no flash or shake. Only transform/opacity animate.
	History: the mode replaces the window's entry (or pushes one), so Back skips the animation, then closes the mode.
	Six effect tiers (decisions 2026-11-09 D): the plan carries the tier of the realm the server's answer reached;
	beats ring / ring2 / nascent / clear / auspicious / gate / galaxy / avatar draw the tier's layers. A successful
	realm breakthrough ends on the static achievement card (realmCard.ts) with an optional image export (lazy).
-->
<script lang="ts">
	import { onDestroy, onMount, untrack } from 'svelte';
	import { pushState, replaceState } from '$app/navigation';
	import { page } from '$app/state';
	import CultivatorFigure from '$lib/components/CultivatorFigure.svelte';
	import FormationCircle from '$lib/components/FormationCircle.svelte';
	import { charArt, charArtFirst } from '$lib/game/charArt.svelte';
	import { cosmetics } from '$lib/game/cosmetics.svelte';
	import { myFaction } from '$lib/game/faction.svelte';
	import { bodySlots, figureOf } from '$lib/game/looks';
	import { hasChiTonSeal, realmCard } from '../fx/realmCard';
	import Icon from '$lib/components/Icon.svelte';
	import { formatPoints } from '$lib/game/format';
	import { game } from '$lib/game/store.svelte';
	import { beatsAt, type BtBeat } from '../fx/breakthrough';
	import { fxLayer } from '../fx/layer';
	import { fxOverlay } from '../fx/overlay.svelte';
	import type { ParticlePreset } from '../fx/particles';
	import { gameLoop } from '../loop';
	import { settings } from '../settings.svelte';
	import { btMode } from './breakthrough.svelte';
	import { gatherQi } from './gather';

	const cat = $derived(game.catalogue!);
	const res = $derived(btMode.result);
	const plan = $derived(btMode.plan);
	// target realm: the answer's stage once known, else the preview the player confirmed
	const toStage = $derived(res?.to ?? game.preview?.to ?? game.character?.stageId ?? '');
	const realm = $derived(cat.realmOf(toStage));
	const aura = $derived(realm?.visual.aura ?? 'jade');
	const stageName = $derived(cat.stages.get(toStage)?.name ?? '');
	const tribName = $derived(game.preview?.tribulation?.name ?? 'Lôi Kiếp');
	const success = $derived(res?.result === 'success');
	// effect tier played (from the plan = the server's `to` realm), else the target realm's tier while gathering
	const tier = $derived(plan?.tier ?? realm?.visual.fxTier ?? 2);
	const gold = $derived(tier >= 5);
	// the player's own figure for the nascent soul / pháp tướng / the card (display only)
	const look = $derived(game.character?.appearance?.look);
	const figure = $derived(figureOf(cosmetics.looks, look));
	// ── achievement card (static end state of a successful realm breakthrough) ──
	const chain = $derived(
		realm ? [...cat.realms.values()].filter((r) => r.path === realm.path && r.tier <= realm.tier).sort((a, b) => b.tier - a.tier).map((r) => r.id) : []
	);
	const bodyArt = $derived(look && realm ? charArtFirst(bodySlots(look, chain, realm.visual.fxTier)) : undefined);
	// tiers 5–6: the painted realm-aura overlay (fx/aura-5 = five-colour clouds round a jade ring, fx/aura-6 = the
	// starlit pháp tướng over a golden ring) carries the celebration; CSS layers stay as the fallback while it loads /
	// if it is missing. Resolved while gathering (the target realm is known) and decoded before the answer lands.
	// auspicious clouds = aura-5 (tiers 5–6), the starlit sky = aura-6 (tier 6)
	const ausArt = $derived(tier >= 5 ? charArt('fx/aura-5') : undefined);
	const skyArt = $derived(tier >= 6 ? charArt('fx/aura-6') : undefined);
	$effect(() => {
		for (const a of [ausArt, skyArt, tier >= 5 ? bodyArt : undefined]) {
			if (!a) continue;
			const im = new Image();
			im.src = a.src;
			void im.decode?.().catch(() => {});
		}
	});
	const card = $derived.by(() => {
		if (!res || !success || !realm) return null;
		const ch = game.character;
		const fac = myFaction();
		return realmCard({
			name: ch?.name ?? '',
			realmName: realm.name,
			stageName,
			fxTier: realm.visual.fxTier,
			// the server's stage date once the character reloaded onto the new stage
			reachedAt: ch && ch.stageId === res.to ? ch.stageReachedAt : null,
			rank: game.myRank,
			title: cosmetics.name(ch?.appearance?.titleId) || undefined,
			faction: fac.state === 'member' ? fac.view?.name : undefined,
			seal: hasChiTonSeal(game.vip?.benefits.cosmetics)
		});
	});
	let cardFig = $state<HTMLElement>();
	let saving = $state(false);
	async function saveCard(e: Event) {
		e.stopPropagation();
		if (!card || saving) return;
		saving = true;
		try {
			const { exportCard } = await import('../fx/cardExport');
			await exportCard(card, cardFig?.querySelector('img, svg') ?? null, aura);
		} catch {
			/* nothing saved: the card stays */
		} finally {
			saving = false;
		}
	}

	// ── timeline ──
	let t = $state(0);
	let beats = $state<Set<BtBeat>>(new Set());
	let strikeNo = $state(0);
	const done = $derived(btMode.phase === 'outcome' && !!plan && t >= plan.totalMs);
	let unsub: (() => void) | null = null;
	const stopClock = () => {
		unsub?.();
		unsub = null;
	};
	function setT(v: number) {
		t = v;
		if (!plan) return;
		const b = beatsAt(plan, v);
		const key = [...b].join();
		if (key !== [...beats].join()) beats = b;
		const n = plan.steps.filter((s) => s.beat === 'strike' && s.at <= v).length;
		if (n !== strikeNo) strikeNo = n;
	}
	$effect(() => {
		void btMode.seq;
		const p = btMode.plan;
		if (btMode.phase !== 'outcome' || !p) return;
		untrack(() => {
			stopClock();
			strikeNo = 0;
			beats = new Set();
			setT(0);
			if (p.totalMs <= 0) return;
			unsub = gameLoop().subscribe((dt) => {
				setT(t + dt);
				if (t >= p.totalMs) stopClock();
			});
		});
		return stopClock;
	});
	const skip = () => {
		if (plan && !done) setT(plan.totalMs);
	};
	const has = (b: BtBeat) => beats.has(b);
	const stepMs = (b: BtBeat) => plan?.steps.find((s) => s.beat === b)?.ms ?? 0;
	// light rays behind the gate / pháp tướng / pillar (tiers 5–6, success only)
	const rays = $derived(tier >= 5 && success && (has('gate') || has('galaxy') || has('avatar') || (has('pillar') && !done)));

	let burstDone = -1;
	// reveal burst in the new realm's aura colour (pooled fxLayer; 0 when off)
	$effect(() => {
		if (!plan || !success || !(has('pillar') || has('clear'))) return;
		untrack(() => {
			if (burstDone === btMode.seq) return;
			burstDone = btMode.seq;
			const preset: ParticlePreset = {
				id: `bt_${aura}`,
				count: plan.particles,
				liteScale: 1,
				lifeMs: [700, 1300],
				speed: [80, 320],
				angle: -Math.PI / 2,
				spread: Math.PI * 2,
				size: [1.4, 3.4],
				gravity: 30,
				drag: 0.5,
				// tiers 5–6 add the auspicious colours to the burst
				colors: tier >= 5 ? [`var(--color-aura-${aura})`, 'var(--color-gold-300)', 'var(--color-spirit-violet)', 'var(--color-spirit-cyan)'] : [`var(--color-aura-${aura})`, 'var(--color-gold-300)']
			};
			fxOverlay.raised++;
			fxLayer.burst(preset, innerWidth / 2, innerHeight * 0.45, plan.particles);
		});
	});
	// gather loop: motes converge on the cultivator while waiting
	$effect(() => {
		const g = btMode.gather;
		const gathering = btMode.phase === 'gather';
		if (!g || !gathering || g.motesPerSec <= 0) return;
		const color = `var(--color-aura-${untrack(() => aura)})`;
		return gatherQi(() => ({ x: innerWidth / 2, y: innerHeight * 0.45 }), settings.fx, color, g.motesPerSec);
	});

	// ── history: Back skips, then closes ──
	let ownEntry = false;
	onMount(() => {
		if (page.state.wins?.length) replaceState('', { mode: 'breakthrough' });
		else pushState('', { mode: 'breakthrough' });
		ownEntry = true;
	});
	$effect(() => {
		const inState = page.state.mode === 'breakthrough';
		if (inState || !ownEntry) return;
		// the entry was popped (Back): skip first, keep the mode until it is closed
		untrack(() => {
			ownEntry = false;
			if (btMode.phase === 'outcome' && !done) {
				skip();
				pushState('', { mode: 'breakthrough' });
				ownEntry = true;
			} else if (btMode.phase === 'outcome' || btMode.phase === 'stall') close();
			else {
				pushState('', { mode: 'breakthrough' }); // gathering: the answer is on its way
				ownEntry = true;
			}
		});
	});
	function close() {
		stopClock();
		const leaveEntry = ownEntry && page.state.mode === 'breakthrough';
		ownEntry = false;
		btMode.close();
		if (leaveEntry) history.back();
	}
	onDestroy(stopClock);

	function onkeydown(e: KeyboardEvent) {
		if (e.key !== 'Escape' && e.key !== ' ' && e.key !== 'Enter') return;
		if (btMode.phase === 'outcome' && !done) {
			e.preventDefault();
			skip();
		} else if (e.key === 'Escape' && (done || btMode.phase === 'stall')) {
			e.preventDefault();
			close();
		}
	}

	const outcomeTitle = $derived.by(() => {
		if (!res) return '';
		if (res.result === 'success') return res.tribulation ? `Vượt qua ${tribName}` : res.kind === 'major' ? 'Đột phá đại cảnh giới' : 'Đột phá thành công';
		return res.tribulation ? 'Độ kiếp thất bại' : 'Đột phá thất bại';
	});
	// exact server values: the loss (0 = intact) and the streak
	const lossText = $derived(res && res.result === 'failure' ? (res.lossMilli > 0 ? `Mất ${formatPoints(res.lossMilli)} tu vi` : 'Tu vi vẫn còn nguyên') : '');
	let cardBtn = $state<HTMLButtonElement>();
	$effect(() => {
		if (done) cardBtn?.focus({ preventScroll: true }); // the card reads from its top (badge, figure, realm)
	});
</script>

<svelte:window {onkeydown} />

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions (Esc/Space/Enter skip on window) -->
<div
	class="bt-mode"
	class:is-dark={btMode.tribulation && (btMode.phase !== 'outcome' || !done || res?.result === 'failure')}
	class:is-fail={!!res && res.result === 'failure'}
	data-fx-mode="breakthrough"
	data-bt-phase={done ? 'result' : btMode.phase}
	data-bt-outcome={plan?.outcome}
	data-bt-tier={tier}
	style:--aura="var(--color-aura-{aura})"
	role="dialog"
	aria-modal="true"
	aria-label="Đột phá"
	tabindex="-1"
	onclick={skip}
>
	<div class="stage" class:is-shake={has('shake')}>
		<div class="sky" aria-hidden="true"></div>
		{#if rays}<span class="rays" aria-hidden="true"></span>{/if}
		{#if has('galaxy')}
			{#if skyArt}
				<!-- painted starlit pháp tướng + golden ring, two parallax depths of the same layer -->
				<div class="sky-art" style:--ms="{stepMs('galaxy')}ms" aria-hidden="true">
					<img class="far" src={skyArt.src} srcset={skyArt.srcset} alt="" draggable="false" />
					<img class="near" src={skyArt.src} srcset={skyArt.srcset} alt="" draggable="false" />
				</div>
			{:else}
				<span class="galaxy" style:--ms="{stepMs('galaxy')}ms" aria-hidden="true"></span>
			{/if}
		{/if}
		{#if has('avatar')}
			<div class="avatar" class:is-art={!!bodyArt} style:--ms="{stepMs('avatar')}ms" aria-hidden="true">
				{#if bodyArt}
					<img src={bodyArt.src} srcset={bodyArt.srcset} alt="" class="size-full object-contain" draggable="false" />
				{:else}
					<CultivatorFigure {aura} {figure} still class="size-full" />
				{/if}
			</div>
		{/if}
		{#if has('auspicious')}
			{#if ausArt}
				<!-- painted five-colour clouds: a soft far layer rising slowly, a crisp near layer rising faster
				     (tier 6: only the soft band — the golden ring of the starlit sky is the focus) -->
				<div class="aus-art" class:t6={tier >= 6} style:--ms="{stepMs('auspicious')}ms" aria-hidden="true">
					<img class="far" src={ausArt.src} srcset={ausArt.srcset} alt="" draggable="false" />
					{#if tier < 6}<img class="near" src={ausArt.src} srcset={ausArt.srcset} alt="" draggable="false" />{/if}
				</div>
			{:else}
				<div class="auspicious" style:--ms="{stepMs('auspicious')}ms" aria-hidden="true"><i></i><i></i><i></i><i></i><i></i></div>
			{/if}
		{/if}
		{#if has('gate')}
			<svg class="gate" class:is-fail={res?.result === 'failure'} viewBox="0 0 240 200" style:--ms="{stepMs('gate')}ms" aria-hidden="true">
				<path class="arch" d="M20 196 V70 Q120 -10 220 70 V196" />
				<rect class="door l" x="30" y="70" width="90" height="126" />
				<rect class="door r" x="120" y="70" width="90" height="126" />
			</svg>
		{/if}
		{#if btMode.tribulation}
			<div class="clouds" class:is-roll={btMode.phase !== 'outcome' || has('darken') || has('strike')} class:is-clear={has('clear') || (done && success)} style:--ms="{stepMs('clear') || 500}ms" aria-hidden="true">
				<span class="cloud c1"></span><span class="cloud c2"></span><span class="cloud c3"></span>
			</div>
		{/if}

		<div class="center" aria-hidden="true">
			<span
				class="formation"
				class:is-turn={btMode.gather?.motion && !done}
				class:is-spin={has('spin')}
				style:--turn="{btMode.gather?.turnMs || 6000}ms"
				style:--ms="{stepMs('spin')}ms"
			><FormationCircle color="var(--aura)" {tier} class="size-full" /></span>
			{#if has('ring2')}
				<span class="formation f2" style:--ms="{stepMs('ring2')}ms"><FormationCircle color="var(--color-gold-300)" {tier} class="size-full" /></span>
			{/if}
			{#if has('ring')}<span class="ring-wave" style:--ms="{stepMs('ring')}ms"></span>{/if}
			{#if has('nascent')}
				<span class="nascent" style:--ms="{stepMs('nascent')}ms"><CultivatorFigure {aura} {figure} crop="bust" still class="size-full" /></span>
			{/if}
			<span class="aura" class:is-gather={btMode.phase === 'gather' && btMode.gather?.motion} class:is-new={has('pillar') || (done && success)} style:--ms="800ms"></span>
			<span class="dantian"></span>
			{#if has('pillar') || (done && success)}<span class="pillar" class:is-static={done} style:--ms="{stepMs('pillar')}ms"></span>{/if}
		</div>

		{#if has('strike')}
			{#key strikeNo}
				<svg viewBox="0 0 200 600" class="bolt" class:b2={strikeNo === 2} class:b3={strikeNo === 3} preserveAspectRatio="none" aria-hidden="true">
					<path d="M110 0 L92 140 L120 150 L78 300 L104 306 L70 470 L96 476 L84 600" />
				</svg>
			{/key}
		{/if}
		{#if plan?.flash && has('flash')}
			{#key strikeNo}<span class="flash" style:--ms="{stepMs('flash')}ms" aria-hidden="true"></span>{/key}
		{/if}
		{#if has('clouds')}
			<span class="part left" style:--ms="{stepMs('clouds')}ms" aria-hidden="true"></span>
			<span class="part right" style:--ms="{stepMs('clouds')}ms" aria-hidden="true"></span>
		{/if}
		{#if has('crack') || (done && res?.result === 'failure')}
			<div class="glass" class:is-static={done} style:--ms="{stepMs('crack')}ms" aria-hidden="true">
				<svg viewBox="0 0 400 400" preserveAspectRatio="xMidYMid slice">
					<path
						d="M214 188 L196 150 L204 122 L178 84 L186 52 L160 14 M214 188 L248 170 L262 140 L300 128 L318 96 L356 82 M214 188 L252 204 L290 198 L314 222 L352 226 L392 248 M214 188 L224 228 L210 262 L232 296 L222 338 L240 384 M214 188 L182 210 L150 204 L126 230 L90 236 L52 262 M214 188 L176 182 L150 160 L112 162 L80 140 L34 146 M178 84 L150 92 M300 128 L312 160 M290 198 L300 172 M210 262 L182 270 M150 204 L148 238 M112 162 L118 132"
					/>
					<circle cx="214" cy="188" r="7" />
				</svg>
			</div>
		{/if}

		{#if has('title') && !done}
			<p class="title-anim font-display is-cal" class:is-gold={gold} style:--ms="{stepMs('title')}ms" aria-hidden="true">
				<span class="ink">{realm?.name ?? ''}</span>
				{#if gold}
					<svg class="brush" viewBox="0 0 400 24" preserveAspectRatio="none"><path d="M6 16 C80 6 160 20 230 12 S350 6 394 13" /></svg>
				{/if}
			</p>
		{/if}
	</div>

	<!-- minimal mode HUD -->
	<div class="mode-hud">
		<p class="eyebrow flex items-center gap-1.5"><Icon name="bolt" size={14} /> {btMode.tribulation ? tribName : 'Đột phá'}</p>
		{#if btMode.phase === 'outcome' && !done}
			<button type="button" class="btn btn-ghost h-10 px-3 text-sm" onclick={(e) => (e.stopPropagation(), skip())} data-bt-skip>Bỏ qua</button>
		{/if}
	</div>

	{#if btMode.phase === 'gather'}
		<div class="caption" aria-live="polite">
			<p class="font-display text-2xl text-gold-300 sm:text-3xl">{btMode.tribulation ? 'Thiên kiếp đang tụ…' : 'Đang tụ khí…'}</p>
			<p class="mt-1 text-sm text-mist-300">Kết quả chỉ do Thiên đạo (máy chủ) phán định.</p>
		</div>
	{:else if btMode.phase === 'stall'}
		<div class="card panel" role="alertdialog" aria-label="Chưa có hồi âm" data-bt-stall>
			<p class="flex items-center justify-center gap-2 font-display text-2xl text-mist-100"><Icon name="clock" size={20} class="text-gold-300" /> {btMode.message || 'Thiên đạo chưa hồi âm.'}</p>
			<p class="mt-2 text-sm text-mist-300">Lần đột phá vẫn được giữ: thử lại sẽ hỏi lại đúng lần này, không tính thêm lượt. Kết quả hiện ngay khi máy chủ trả lời.</p>
			<div class="mt-4 grid gap-2 sm:grid-cols-2">
				<button type="button" class="btn btn-jade" onclick={(e) => (e.stopPropagation(), void game.attempt())}>Thử lại</button>
				<button type="button" class="btn btn-ghost" onclick={(e) => (e.stopPropagation(), close())}>Về</button>
			</div>
		</div>
	{:else if done && res}
		<div class="card panel" class:is-static={settings.fx === 'off'} data-bt-result={res.result}>
			<p class="badge" class:is-pass={success}>
				<Icon name={success ? 'check' : 'cross'} size={16} />
				{success ? 'Thành công' : 'Thất bại'}
			</p>
			<p class="mt-3 text-sm font-medium tracking-[0.3em] text-gold-300 uppercase">{outcomeTitle}</p>
			{#if success && card}
				<div class="achv mt-3" data-realm-card data-card-tier={card.fxTier}>
					<div bind:this={cardFig} class="achv-fig" style:--aura="var(--color-aura-{aura})">
						{#if bodyArt}
							<img src={bodyArt.src} srcset={bodyArt.srcset} alt="" class="size-full object-contain" draggable="false" />
						{:else}
							<CultivatorFigure {aura} {figure} still class="size-full" />
						{/if}
					</div>
					<p class="realm font-display text-5xl font-bold sm:text-6xl">{card.realmName}</p>
					<p class="mt-1 font-display text-2xl text-mist-100">{card.stageName}</p>
					<p class="mt-2 font-display text-xl text-mist-100">{card.name}{card.title ? ` 「${card.title}」` : ''}</p>
					<p class="mt-1 text-sm text-mist-300">{[card.faction, card.rank, card.date].filter(Boolean).join(' · ')}</p>
					{#if card.seal}<span class="seal font-display" title="Ấn Chí Tôn (Tiên Duyên) — chỉ là dấu trang trí" data-chi-ton-seal>Chí<br />Tôn</span>{/if}
				</div>
				<button type="button" class="btn btn-ghost mt-4 h-11 w-full text-sm" onclick={saveCard} disabled={saving} aria-busy={saving || undefined} data-card-save>
					<Icon name="scroll" size={16} /> {saving ? 'Đang lưu…' : 'Lưu ảnh thẻ thành tựu'}
				</button>
			{:else if success}
				<p class="realm mt-2 font-display text-5xl font-bold sm:text-6xl">{realm?.name ?? ''}</p>
				<p class="mt-2 font-display text-2xl text-mist-100">{stageName}</p>
			{:else}
				<p class="mt-3 font-display text-3xl font-bold text-cinnabar-300" data-bt-loss>{lossText}</p>
				<p class="mt-2 text-mist-300">Cảnh giới giữ nguyên{res.failStreak > 0 ? ` · thất bại ${res.failStreak} lần liên tiếp` : ''}.</p>
				{#if !res.tribulation}<p class="mt-1 text-sm text-mist-500">Tâm cảnh vững hơn: lần sau tỉ lệ sẽ tăng.</p>{/if}
			{/if}
			<button bind:this={cardBtn} type="button" class="btn btn-gold mt-3 h-12 w-full" onclick={(e) => (e.stopPropagation(), close())} data-bt-close>Tiếp tục</button>
			<span class="sr-only" aria-live="polite">{outcomeTitle}. {success ? `${realm?.name ?? ''} ${stageName}` : lossText}</span>
		</div>
	{/if}
</div>

<style>
	.bt-mode {
		position: fixed;
		inset: 0;
		z-index: var(--z-local-2);
		overflow: hidden;
		pointer-events: auto;
		background: radial-gradient(circle at 50% 45%, color-mix(in oklab, var(--color-ink-800) 94%, transparent), var(--color-ink-950));
		animation: bt-in var(--dur-base) var(--ease-qi);
		outline: none;
	}
	@keyframes bt-in {
		from {
			opacity: 0;
		}
	}
	.stage {
		position: absolute;
		inset: 0;
	}
	.stage.is-shake {
		animation: shake 200ms linear;
	}
	@keyframes shake {
		25% {
			transform: translate(-6px, 3px);
		}
		50% {
			transform: translate(5px, -4px);
		}
		75% {
			transform: translate(-3px, -2px);
		}
	}
	.sky {
		position: absolute;
		inset: 0;
		background: radial-gradient(ellipse at 50% 0%, color-mix(in oklab, var(--color-spirit-violet) 30%, transparent), transparent 60%), var(--color-ink-950);
		opacity: 0;
		transition: opacity var(--dur-slow) var(--ease-qi);
	}
	.is-dark .sky {
		opacity: 0.92;
	}
	.clouds {
		position: absolute;
		inset: 0 0 auto;
		height: 45%;
	}
	.cloud {
		position: absolute;
		top: 0;
		width: 70%;
		height: 60%;
		border-radius: 50%;
		/* closest-side: the gradient fades out inside the ellipse (no hard rim against bright layers behind) */
		background: radial-gradient(closest-side at 50% 55%, var(--color-ink-950), color-mix(in oklab, var(--color-ink-900) 70%, transparent) 60%, transparent);
		opacity: 0.9;
	}
	.c1 {
		left: -20%;
	}
	.c2 {
		left: 30%;
		top: 8%;
	}
	.c3 {
		left: 60%;
		top: -6%;
	}
	:global(:root[data-fx='full']) .clouds.is-roll .cloud {
		animation: roll 5s var(--ease-in-out-sine) infinite alternate;
	}
	:global(:root[data-fx='full']) .clouds.is-roll .c2 {
		animation-duration: 3.6s;
		animation-direction: alternate-reverse;
	}
	@keyframes roll {
		to {
			transform: translateX(14%) scale(1.08);
		}
	}
	.center {
		position: absolute;
		left: 50%;
		top: 45%;
		width: min(78vw, 46vh, 460px);
		aspect-ratio: 1;
		transform: translate(-50%, -50%);
	}
	.formation {
		position: absolute;
		inset: 0;
		opacity: 0.7;
		transform: perspective(600px) rotateX(58deg);
	}
	.formation :global(svg) {
		display: block;
	}
	.formation.is-turn :global(svg) {
		animation: turn var(--turn) linear infinite;
	}
	.formation.is-spin :global(svg) {
		animation: turn-fast var(--ms) cubic-bezier(0.55, 0, 1, 0.45) forwards;
	}
	@keyframes turn {
		to {
			transform: rotate(360deg);
		}
	}
	@keyframes turn-fast {
		to {
			transform: rotate(1080deg);
		}
	}
	.aura {
		position: absolute;
		inset: 18%;
		border-radius: 50%;
		background: radial-gradient(circle, color-mix(in oklab, var(--aura) 60%, transparent), transparent 68%);
		opacity: 0.55;
	}
	.aura.is-gather {
		animation: gather var(--ms) var(--ease-in-out-sine) infinite alternate;
	}
	.aura.is-new {
		opacity: 1;
		transform: scale(1.35);
		animation: aura-new 700ms var(--ease-out-back);
	}
	@keyframes gather {
		from {
			transform: scale(1.25);
			opacity: 0.25;
		}
		to {
			transform: scale(0.7);
			opacity: 0.9;
		}
	}
	@keyframes aura-new {
		from {
			transform: scale(0.6);
			opacity: 0.3;
		}
	}
	.dantian {
		position: absolute;
		left: 50%;
		top: 50%;
		width: 18px;
		height: 18px;
		margin: -9px;
		border-radius: 50%;
		background: var(--color-mist-100);
		box-shadow: 0 0 24px 8px var(--aura);
	}
	.pillar {
		position: absolute;
		left: 50%;
		bottom: 50%;
		width: 34%;
		height: 100vh;
		transform: translateX(-50%);
		transform-origin: bottom;
		background: linear-gradient(90deg, transparent, color-mix(in oklab, var(--aura) 45%, transparent), color-mix(in oklab, var(--color-mist-100) 70%, transparent), color-mix(in oklab, var(--aura) 45%, transparent), transparent);
		animation: pillar var(--ms) var(--ease-qi);
	}
	.pillar.is-static {
		opacity: 0.25;
		animation: none;
	}
	@keyframes pillar {
		from {
			transform: translateX(-50%) scaleY(0);
		}
	}
	.bolt {
		position: absolute;
		top: 0;
		left: calc(50% - 90px);
		width: 140px;
		height: 58%;
		fill: none;
		stroke: var(--color-mist-100);
		stroke-width: 3;
		stroke-linejoin: bevel;
		filter: drop-shadow(0 0 6px var(--color-spirit-cyan)) drop-shadow(0 0 16px var(--color-spirit-cyan));
		animation: strike 240ms steps(1) forwards;
	}
	.bolt.b2 {
		left: calc(50% + 10px);
		height: 50%;
		transform: scaleX(-1);
	}
	.bolt.b3 {
		left: calc(50% - 40px);
		height: 62%;
	}
	@keyframes strike {
		0% {
			opacity: 1;
		}
		30% {
			opacity: 0.25;
		}
		55% {
			opacity: 1;
		}
		100% {
			opacity: 0;
		}
	}
	.flash {
		position: absolute;
		inset: 0;
		background: var(--color-mist-100);
		opacity: 0;
		animation: flash var(--ms) ease-out;
	}
	:global(:root[data-fx='lite']) .flash {
		animation-name: flash-lite;
	}
	@keyframes flash {
		from {
			opacity: 0.55;
		}
	}
	@keyframes flash-lite {
		from {
			opacity: 0.25;
		}
	}
	.part {
		position: absolute;
		top: 0;
		width: 60%;
		height: 50%;
		opacity: 0;
		background: radial-gradient(ellipse at 50% 50%, color-mix(in oklab, var(--color-mist-300) 35%, transparent), transparent 65%);
		animation: part var(--ms) var(--ease-qi);
	}
	.part.left {
		left: -5%;
		--dir: -60%;
	}
	.part.right {
		right: -5%;
		--dir: 60%;
	}
	@keyframes part {
		from {
			opacity: 1;
			transform: none;
		}
		to {
			opacity: 0;
			transform: translateX(var(--dir));
		}
	}
	.glass {
		position: absolute;
		inset: 0;
		backdrop-filter: blur(3px) saturate(0.7);
		background: color-mix(in oklab, var(--color-mist-100) 6%, transparent);
		animation: glass var(--ms) var(--ease-qi);
	}
	.glass.is-static {
		animation: none;
	}
	.glass svg {
		width: 100%;
		height: 100%;
		fill: none;
		stroke: color-mix(in oklab, var(--color-mist-100) 55%, transparent);
		stroke-width: 1.1;
		stroke-linejoin: bevel;
	}
	@keyframes glass {
		from {
			opacity: 0;
			transform: scale(1.04);
		}
	}
	.title-anim {
		position: absolute;
		left: 0;
		right: 0;
		top: 18%;
		text-align: center;
		font-size: clamp(2.5rem, 9vw, 6rem);
		font-weight: 700;
		color: var(--aura);
		text-shadow: 0 0 24px color-mix(in oklab, var(--aura) 60%, transparent);
		animation: title var(--ms) var(--ease-qi);
	}
	@keyframes title {
		from {
			opacity: 0;
			transform: translateY(10px) scale(1.12);
		}
	}
	.mode-hud {
		position: absolute;
		inset: 0 0 auto;
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 0.75rem;
		padding: 0.75rem 1rem;
		min-height: 3.5rem;
	}
	.caption {
		position: absolute;
		left: 0;
		right: 0;
		bottom: 12%;
		padding: 0 1rem;
		text-align: center;
	}
	.card {
		position: absolute;
		left: 50%;
		top: 50%;
		width: min(92vw, 28rem);
		max-height: 90dvh;
		overflow-y: auto;
		transform: translate(-50%, -50%);
		padding: 1.5rem 1.25rem;
		text-align: center;
		/* opaque enough that the pillar / formation never sit behind the text */
		background: color-mix(in oklab, var(--color-ink-950) 90%, transparent);
		backdrop-filter: blur(6px);
		animation: card-in var(--dur-slow) var(--ease-qi);
	}
	@keyframes card-in {
		from {
			opacity: 0;
			transform: translate(-50%, -46%);
		}
	}
	.badge {
		display: inline-flex;
		align-items: center;
		gap: 0.35rem;
		padding: 3px 12px;
		border: 1px solid currentColor;
		border-radius: 9999px;
		font-weight: 600;
		font-size: 0.85rem;
		color: var(--color-cinnabar-300);
	}
	.badge.is-pass {
		color: var(--color-gold-300);
	}
	.realm {
		color: var(--aura);
		text-shadow: 0 0 24px color-mix(in oklab, var(--aura) 50%, transparent);
	}

	/* ── effect tiers (decisions 2026-11-09 D): transient layers, only while their beat runs ── */
	.formation.f2 {
		opacity: 0.5;
		transform: perspective(600px) rotateX(58deg) scale(0.72);
	}
	.formation.f2 :global(svg) {
		animation: turn-back var(--ms) linear forwards;
	}
	@keyframes turn-back {
		to {
			transform: rotate(-540deg);
		}
	}
	.ring-wave {
		position: absolute;
		inset: 10%;
		border-radius: 50%;
		border: 3px solid color-mix(in oklab, var(--aura) 80%, var(--color-mist-100));
		box-shadow: 0 0 22px var(--aura);
		opacity: 0;
		animation: ring-wave var(--ms) var(--ease-qi);
	}
	@keyframes ring-wave {
		from {
			opacity: 0.9;
			transform: scale(0.35);
		}
		to {
			opacity: 0;
			transform: scale(1.6);
		}
	}
	.nascent {
		position: absolute;
		left: 34%;
		top: -38%;
		width: 32%;
		aspect-ratio: 1;
		opacity: 0.45;
		filter: drop-shadow(0 0 16px var(--aura));
		animation: rise var(--ms) var(--ease-out-back);
	}
	@keyframes rise {
		from {
			opacity: 0;
			transform: translateY(30%) scale(0.6);
		}
	}
	.galaxy {
		position: absolute;
		left: 50%;
		top: 45%;
		width: min(150vw, 150vh);
		aspect-ratio: 1;
		border-radius: 50%;
		transform: translate(-50%, -50%);
		background:
			radial-gradient(circle, transparent 18%, color-mix(in oklab, var(--color-ink-950) 20%, transparent) 60%, transparent 70%),
			conic-gradient(from 0deg, color-mix(in oklab, var(--color-spirit-violet) 40%, transparent), transparent 25%, color-mix(in oklab, var(--aura) 45%, transparent) 50%, transparent 75%, color-mix(in oklab, var(--color-spirit-violet) 40%, transparent));
		opacity: 0.38;
		/* soft vignette: the swirl fades into the night instead of reading as a grey bubble (raster fx art later) */
		mask: radial-gradient(circle, black 12%, transparent 52%);
		animation: galaxy var(--ms) linear forwards;
	}
	@keyframes galaxy {
		from {
			opacity: 0;
			transform: translate(-50%, -50%) rotate(0deg) scale(0.6);
		}
		20% {
			opacity: 0.38;
		}
		to {
			transform: translate(-50%, -50%) rotate(200deg) scale(1);
		}
	}
	.avatar {
		position: absolute;
		left: 50%;
		bottom: 18%;
		width: min(120vw, 95vh);
		aspect-ratio: 1;
		transform: translateX(-50%);
		opacity: 0.16;
		filter: drop-shadow(0 0 40px var(--aura));
		animation: avatar var(--ms) var(--ease-qi);
	}
	@keyframes avatar {
		from {
			opacity: 0;
			transform: translateX(-50%) scale(0.5);
		}
	}
	.auspicious {
		position: absolute;
		inset: auto 0 0;
		height: 34%;
		animation: drift var(--ms) var(--ease-qi);
	}
	.auspicious i {
		position: absolute;
		bottom: 0;
		width: 46%;
		height: 60%;
		border-radius: 50%;
		filter: blur(10px);
		opacity: 0.55;
	}
	.auspicious i:nth-child(1) { left: -6%; background: color-mix(in oklab, var(--color-aura-jade) 60%, transparent); }
	.auspicious i:nth-child(2) { left: 16%; bottom: 10%; background: color-mix(in oklab, var(--color-aura-golden) 60%, transparent); }
	.auspicious i:nth-child(3) { left: 36%; background: color-mix(in oklab, var(--color-aura-azure) 60%, transparent); }
	.auspicious i:nth-child(4) { left: 56%; bottom: 8%; background: color-mix(in oklab, var(--color-aura-violet) 60%, transparent); }
	.auspicious i:nth-child(5) { left: 74%; background: color-mix(in oklab, var(--color-aura-crimson) 55%, transparent); }
	@keyframes drift {
		from {
			opacity: 0;
			transform: translateY(30%);
		}
	}
	.gate {
		position: absolute;
		left: 50%;
		top: 4%;
		width: min(70vw, 420px);
		transform: translateX(-50%);
		fill: none;
		stroke: var(--color-gold-300);
		stroke-width: 2;
		filter: drop-shadow(0 0 14px var(--color-gold-300));
	}
	.gate .door {
		fill: color-mix(in oklab, var(--color-gold-500) 22%, var(--color-ink-900));
		transform-box: fill-box;
		animation: door-open var(--ms) var(--ease-qi) forwards;
	}
	.gate .door.l {
		transform-origin: 0 50%;
	}
	.gate .door.r {
		transform-origin: 100% 50%;
	}
	.gate.is-fail .door {
		animation-name: door-shut;
	}
	@keyframes door-open {
		to {
			transform: scaleX(0.12);
		}
	}
	@keyframes door-shut {
		from {
			transform: scaleX(0.6);
		}
		to {
			transform: scaleX(1);
		}
	}
	.clouds.is-clear .cloud {
		animation: cloud-clear var(--ms) var(--ease-qi) forwards;
	}
	.clouds.is-clear .c1 { --dx: -60%; }
	.clouds.is-clear .c2 { --dx: 0%; }
	.clouds.is-clear .c3 { --dx: 60%; }
	@keyframes cloud-clear {
		to {
			opacity: 0;
			transform: translate(var(--dx), -40%);
		}
	}
	:global(:root[data-fx='off']) .clouds.is-clear .cloud {
		opacity: 0;
	}
	.title-anim.is-gold {
		top: 14%;
		font-size: clamp(3rem, 12vw, 8rem);
		color: var(--color-gold-300);
		text-shadow: 0 0 30px color-mix(in oklab, var(--color-gold-300) 70%, transparent), 0 0 60px color-mix(in oklab, var(--aura) 50%, transparent);
	}
	/* ── tiers 5–6 painted layers (fx/aura-5|6): parallax depths, light rays, gold calligraphy ── */
	.rays {
		position: absolute;
		left: 50%;
		top: 40%;
		width: max(160vw, 160vh);
		aspect-ratio: 1;
		transform: translate(-50%, -50%);
		background: repeating-conic-gradient(
			from 0deg,
			color-mix(in oklab, var(--color-gold-300) 30%, transparent) 0deg 4deg,
			transparent 4deg 15deg
		);
		mask: radial-gradient(circle, black 4%, transparent 46%);
		opacity: 0.55;
		animation: rays-in var(--dur-slow) var(--ease-qi);
	}
	:global(:root[data-fx='full']) .rays {
		animation:
			rays-in var(--dur-slow) var(--ease-qi),
			rays-turn 9s linear infinite;
	}
	@keyframes rays-in {
		from {
			opacity: 0;
		}
	}
	@keyframes rays-turn {
		to {
			transform: translate(-50%, -50%) rotate(40deg);
		}
	}
	.aus-art,
	.sky-art {
		position: absolute;
		inset: 0;
		pointer-events: none;
	}
	.aus-art img,
	.sky-art img {
		position: absolute;
		left: 50%;
		max-width: none;
		aspect-ratio: 1;
		object-fit: contain;
	}
	/* five-colour clouds: the painting's lower band frames the bottom of the screen */
	.aus-art .far {
		bottom: -38%;
		width: max(150vw, 120vh);
		opacity: 0.35;
		filter: blur(3px);
		transform: translateX(-50%);
		animation: aus-far var(--ms) var(--ease-qi) both;
	}
	.aus-art .near {
		bottom: -30%;
		width: max(118vw, 92vh);
		opacity: 0.9;
		transform: translateX(-50%);
		animation: aus-near var(--ms) var(--ease-out-back) both;
	}
	.aus-art.t6 .far {
		opacity: 0.5;
	}
	/* portrait phones: the cloud ring sits in view near the bottom */
	@media (orientation: portrait) {
		.aus-art .near {
			bottom: -6%;
			width: 150vw;
		}
		.aus-art .far {
			bottom: -8%;
			width: 230vw;
		}
	}
	@keyframes aus-far {
		from {
			opacity: 0;
			transform: translate(-50%, 6%) scale(1.08);
		}
	}
	@keyframes aus-near {
		from {
			opacity: 0;
			transform: translate(-50%, 16%) scale(0.92);
		}
	}
	/* tier 6: the starlit pháp tướng over its golden ring rises behind the formation; a soft far copy drifts */
	.sky-art .far {
		top: 50%;
		width: min(150vw, 135vh);
		opacity: 0.22;
		filter: blur(4px);
		transform: translate(-50%, -52%) scale(1.15);
		animation: sky-far var(--ms) linear both;
	}
	.sky-art .near {
		top: 46%;
		width: min(118vw, 100vh);
		opacity: 0.7;
		transform: translate(-50%, -50%);
		animation: sky-near var(--ms) var(--ease-qi) both;
	}
	@keyframes sky-far {
		from {
			opacity: 0;
			transform: translate(-50%, -48%) scale(1.3);
		}
	}
	@keyframes sky-near {
		from {
			opacity: 0;
			transform: translate(-50%, -42%) scale(0.86);
		}
	}
	/* the player's own painted figure as the giant pháp tướng (ghostly, lit from within) */
	.avatar.is-art {
		height: min(84vh, 140vw);
		width: auto;
		aspect-ratio: 2 / 3;
		bottom: 6%;
		opacity: 0.38;
		mix-blend-mode: screen;
		filter: drop-shadow(0 0 30px var(--color-gold-300)) drop-shadow(0 0 60px var(--aura)) brightness(1.25);
	}
	/* gold calligraphy: a gradient ink revealed left → right like a brush stroke, a brush underline after it */
	.title-anim.is-gold .ink {
		display: inline-block;
		padding: 0 0.15em;
		letter-spacing: 0.06em;
		background: linear-gradient(180deg, var(--color-mist-100) 10%, var(--color-gold-300) 45%, var(--color-gold-500) 90%);
		background-clip: text;
		-webkit-background-clip: text;
		color: transparent;
		filter: drop-shadow(0 0 18px color-mix(in oklab, var(--color-gold-300) 60%, transparent)) drop-shadow(0 2px 0 var(--color-ink-950));
		animation: ink-reveal var(--ms) var(--ease-qi) both;
	}
	.title-anim.is-gold {
		text-shadow: none;
	}
	.title-anim .brush {
		display: block;
		margin: -0.1em auto 0;
		width: min(70vw, 34rem);
		height: 0.32em;
		fill: none;
		stroke: var(--color-gold-300);
		stroke-width: 5;
		stroke-linecap: round;
		stroke-dasharray: 420;
		stroke-dashoffset: 0;
		filter: drop-shadow(0 0 8px var(--color-gold-300));
		animation: brush var(--ms) var(--ease-qi) both;
	}
	@keyframes ink-reveal {
		from {
			clip-path: inset(0 100% 0 0);
			transform: scale(1.08);
		}
		60% {
			clip-path: inset(0 0 0 0);
		}
	}
	@keyframes brush {
		0%,
		45% {
			stroke-dashoffset: 420;
		}
	}
	/* ── achievement card ── */
	.achv {
		position: relative;
	}
	.achv-fig {
		margin: 0 auto 0.5rem;
		width: min(52vw, 15rem);
		height: min(36vh, 15rem);
		filter: drop-shadow(0 0 22px color-mix(in oklab, var(--aura) 55%, transparent));
	}
	.seal {
		position: absolute;
		right: 0.25rem;
		bottom: 0;
		display: grid;
		place-items: center;
		width: 3.4rem;
		height: 3.4rem;
		font-size: 0.95rem;
		font-weight: 700;
		line-height: 1.05;
		color: var(--color-gold-300);
		border: 2px solid var(--color-gold-300);
		transform: rotate(-7deg);
		opacity: 0.85;
	}
	/* landscape phones: a compact card (figure and realm name smaller) so most of it fits without scrolling */
	@media (max-height: 520px) {
		.achv-fig {
			height: 24vh;
		}
		.achv .realm {
			font-size: 2.25rem;
		}
	}
</style>
