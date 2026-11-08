<!--
	Onboarding (data-model §9.6, plan "Luồng người mới"): opening → Tử Tiêu Cung (Hồng Quân's opening words) →
	faction → path (only when the faction allows several) → đạo hiệu. The steps are client-side; the last one sends ONE
	intent, POST /character {name, factionId, path}, which creates the character and joins the faction in one server
	transaction, then lands in Tử Tiêu Cung. The draft (step, choices, Idempotency-Key) lives in sessionStorage so a
	reload doesn't replay the opening and a retry reuses the same key.
-->
<script lang="ts">
	import { goto, invalidateAll } from '$app/navigation';
	import { MediaQuery } from 'svelte/reactivity';
	import { fade, fly } from 'svelte/transition';
	import { api, ApiError, newIdempotencyKey } from '$lib/api/client';
	import FactionEmblem from '$lib/components/FactionEmblem.svelte';
	import FactionPicker from '$lib/components/FactionPicker.svelte';
	import Icon from '$lib/components/Icon.svelte';
	import OpeningSequence from '$lib/components/OpeningSequence.svelte';
	import { art } from '$lib/game/art';
	import { factionColor, factionError } from '$lib/game/faction.svelte';
	import {
		emptyDraft,
		nextStep,
		openingWords,
		prevStep,
		resolvePath,
		sanitizeDraft,
		stepLabels,
		stepsFor,
		type Draft,
		type Step
	} from '$lib/game/onboarding';
	import { toast } from '$lib/game/toasts.svelte';
	import type { FactionView, PathId } from '$lib/game/types';

	let { data } = $props();

	const STORE = 'tiendao:onboarding';
	// Svelte transitions use the Web Animations API, which the global reduced-motion CSS rule doesn't reach.
	const reduced = new MediaQuery('(prefers-reduced-motion: reduce)');
	const dur = (ms: number) => (reduced.current ? 0 : ms);
	const hallArt = art('scenes/tu-tieu-cung');
	const sageArt = art('characters/hong-quan');

	const open = $derived(data.list.factions.filter((f) => f.open));
	const pathName = (p: PathId) => data.catalogue.paths.get(p)?.name ?? p;
	const cooldownDays = $derived(Math.round(data.list.switchCooldownSeconds / 86_400));

	function restore(): Draft {
		try {
			const raw = sessionStorage.getItem(STORE);
			if (raw) return sanitizeDraft({ ...emptyDraft(), ...(JSON.parse(raw) as Partial<Draft>) }, data.list.factions);
		} catch {
			/* private mode / bad JSON → start over */
		}
		return emptyDraft();
	}
	// svelte-ignore state_referenced_locally
	let draft = $state<Draft>(restore());
	$effect(() => {
		const snapshot = JSON.stringify(draft);
		try {
			sessionStorage.setItem(STORE, snapshot);
		} catch {
			/* not persisted: the wizard still works */
		}
	});

	const faction = $derived(open.find((f) => f.id === draft.factionId));
	const steps = $derived(stepsFor(faction));
	const shown = $derived(steps.filter((s) => s !== 'mo-dau'));
	const go = (step: Step) => {
		draft.step = step;
		error = null;
	};
	const forward = () => go(nextStep(draft.step, faction));
	const back = () => go(prevStep(draft.step, faction));

	// ── Tử Tiêu Cung: Hồng Quân's opening words, one tap per line ──
	let line = $state(0);
	const lastLine = $derived(line >= openingWords.length - 1);

	// ── faction / path ──
	function pick(f: FactionView) {
		if (f.id !== draft.factionId) draft.key = null; // another intent → another Idempotency-Key
		draft.factionId = f.id;
		draft.path = resolvePath(f, null);
		go(nextStep('the-luc', f));
	}

	// ── đạo hiệu + the one intent ──
	let error = $state<string | null>(null);
	let busy = $state(false);
	const nameMessages: Record<string, string> = {
		name_taken: 'Đạo hiệu này đã có người dùng, hãy chọn tên khác.',
		character_exists: 'Tài khoản này đã có nhân vật.'
	};

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		if (!faction) return go('the-luc');
		busy = true;
		error = null;
		draft.key ??= newIdempotencyKey();
		try {
			await api.onboard(draft.name.trim(), faction.id, draft.key, resolvePath(faction, draft.path) ?? undefined);
			try {
				sessionStorage.removeItem(STORE);
			} catch {
				/* ignore */
			}
			toast('ok', `${draft.name.trim()} đã ngồi xuống trong Tử Tiêu Cung. Lời giảng đang bắt đầu.`, 6000);
			await goto('/play/tu-tieu-cung', { replaceState: true, invalidateAll: true });
		} catch (err) {
			if (!(err instanceof ApiError) || err.status >= 500) {
				error = 'Không kết nối được máy chủ, hãy thử lại.'; // keep the key: the retry replays the same intent
			} else {
				draft.key = null;
				if (['faction_closed', 'unknown_faction', 'path_not_allowed'].includes(err.code)) {
					draft.factionId = null;
					go('the-luc');
					error = factionError(err);
				} else if (err.code === 'character_exists') {
					await invalidateAll(); // the guard routes an existing character
				} else {
					error = nameMessages[err.code] ?? err.message;
				}
			}
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head><title>{stepLabels[draft.step]} · Tiên Đạo</title></svelte:head>

{#if draft.step === 'mo-dau'}
	<OpeningSequence factions={open} onfinish={forward} />
{:else}
	<div class="wizard relative h-dvh overflow-y-auto">
		<div class="backdrop" aria-hidden="true">
			{#if hallArt}<img src={hallArt.src} srcset={hallArt.srcset} alt="" draggable="false" />{/if}
		</div>

		<nav class="sticky top-0 z-(--z-local-2) flex items-center gap-3 px-4 py-3 sm:px-8" aria-label="Các bước">
			<button type="button" class="btn btn-ghost h-10 w-10 shrink-0 p-0" onclick={back} aria-label="Quay lại">
				<Icon name="arrow" size={18} class="rotate-180" />
			</button>
			<ol class="flex min-w-0 flex-1 items-center gap-1 text-xs sm:gap-2 sm:text-sm">
				{#each shown as s, i (s)}
					{@const at = shown.indexOf(draft.step)}
					<li class="flex min-w-0 items-center gap-1 sm:gap-2" aria-current={s === draft.step ? 'step' : undefined}>
						{#if i > 0}<span class="h-px w-3 shrink-0 bg-ink-600 sm:w-6" aria-hidden="true"></span>{/if}
						<span class="dot" class:is-done={i < at} class:is-now={s === draft.step}>{i < at ? '✓' : i + 1}</span>
						<span class={['truncate', s === draft.step ? 'text-gold-300' : 'hidden text-mist-500 sm:inline']}>{stepLabels[s]}</span>
					</li>
				{/each}
			</ol>
		</nav>

		{#key draft.step}
			<main class="relative mx-auto max-w-7xl px-4 pb-10 sm:px-8" in:fly={{ y: 16, duration: dur(450) }}>
				{#if draft.step === 'tu-tieu-cung'}
					<section class="hall mx-auto flex max-w-2xl flex-col items-center text-center sm:min-h-[calc(100dvh-8rem)] sm:justify-center">
						<p class="eyebrow mt-2">Tử Tiêu Cung · buổi giảng đầu tiên</p>
						<div class="sage" aria-hidden="true">
							<span class="halo"></span>
							{#if sageArt}
								<img src={sageArt.src} srcset={sageArt.srcset} alt="" draggable="false" />
							{:else}
								<Icon name="spark" size={56} class="text-gold-300" />
							{/if}
						</div>
						<button type="button" class="speech panel w-full p-5 text-left sm:p-6" onclick={() => (lastLine ? forward() : (line += 1))}>
							<span class="flex items-baseline justify-between gap-3">
								<span class="font-display text-xl font-bold text-gold-300">Hồng Quân</span>
								<span class="text-xs text-mist-500 tabular-nums">{line + 1}/{openingWords.length}</span>
							</span>
							{#key line}
								<span class="mt-2 block text-lg leading-relaxed text-mist-100" in:fade={{ duration: dur(350) }} aria-live="polite">
									“{openingWords[line]}”
								</span>
							{/key}
							{#if !lastLine}<span class="mt-3 block text-xs text-mist-500">Chạm để nghe tiếp</span>{/if}
						</button>
						<div class="mt-4 flex w-full flex-col gap-2 sm:flex-row sm:justify-center">
							{#if lastLine}
								<button type="button" class="btn btn-jade h-12 sm:px-8" onclick={forward}>Ra ngoài điện chọn thế lực</button>
							{:else}
								<button type="button" class="btn btn-jade h-12 sm:px-8" onclick={() => (line += 1)}>Nghe tiếp</button>
								<button type="button" class="btn btn-ghost h-12 sm:px-6" onclick={forward}>Bỏ qua lời giảng</button>
							{/if}
						</div>
						<button type="button" class="mt-3 text-sm text-mist-500 underline-offset-4 hover:text-mist-300 hover:underline"
							onclick={() => { line = 0; go('mo-dau'); }}>Xem lại mở đầu</button>
					</section>
				{:else if draft.step === 'the-luc'}
					<header class="mb-6 text-center">
						<p class="eyebrow">Ngoài Tử Tiêu Cung · các thế lực thu nhận đệ tử</p>
						<h1 class="mt-2 text-3xl font-bold text-gold-300 sm:text-5xl">Chọn nơi lòng ngươi thấy yên</h1>
						<p class="mx-auto mt-3 max-w-2xl text-sm text-mist-300 sm:text-base">
							Thế lực định con đường tu của bạn, nơi bạn kết giao và đi qua các đại kiếp. Không thế lực nào mạnh hơn thế lực nào.
						</p>
					</header>
					{#if error}<p role="alert" class="mb-4 text-center text-cinnabar-300">{error}</p>{/if}
					{#if open.length}
						<FactionPicker factions={open} current={null} path="tien" {pathName} newcomer
							verb="Chọn" onjoin={pick}
							note={cooldownDays ? `Sau khi vào game, phải chờ ${cooldownDays} ngày mới đổi phe được.` : undefined} />
					{:else}
						<p class="panel p-6 text-center text-mist-300">Chưa thế lực nào mở cửa. Hãy quay lại sau ít phút.</p>
					{/if}
				{:else if draft.step === 'dao-do' && faction}
					<section class="mx-auto max-w-xl">
						<h1 class="text-center text-3xl font-bold text-gold-300 sm:text-4xl">Chọn đạo đồ</h1>
						<p class="mt-2 text-center text-mist-300">{faction.name} nhận nhiều con đường. Chọn con đường bạn muốn bắt đầu.</p>
						<div class="mt-6 grid gap-3" role="radiogroup" aria-label="Đạo đồ">
							{#each faction.allowedPaths as p (p)}
								<button type="button" role="radio" aria-checked={draft.path === p} class="panel flex h-14 items-center justify-between px-4"
									class:is-picked={draft.path === p} onclick={() => (draft.path = p)}>
									<span class="font-display text-xl font-bold">{pathName(p)}</span>
									{#if draft.path === p}<Icon name="check" size={18} class="text-jade-300" />{/if}
								</button>
							{/each}
						</div>
						<button type="button" class="btn btn-jade mt-6 h-12 w-full" disabled={!draft.path} onclick={forward}>Xác nhận</button>
					</section>
				{:else if draft.step === 'dao-hieu' && faction}
					<form onsubmit={submit} class="mx-auto max-w-md space-y-5">
						<header class="text-center">
							<p class="eyebrow">Bước cuối</p>
							<h1 class="mt-1 text-3xl font-bold text-gold-300 sm:text-4xl">Lập đạo hiệu</h1>
							<p class="mt-2 text-sm text-mist-300">
								Mỗi tài khoản có một nhân vật. Đạo hiệu sẽ được khắc trên Thiên Bảng, hãy chọn kỹ.
							</p>
						</header>
						<div class="chosen panel flex items-center gap-4 p-4" style:--fc={factionColor(faction)}>
							<span class="shrink-0" style:color="var(--fc)"><FactionEmblem id={faction.id} size={56} /></span>
							<div class="min-w-0 flex-1">
								<p class="eyebrow">Thế lực · đạo đồ</p>
								<p class="truncate font-display text-xl font-bold" style:color="var(--fc)">{faction.name}</p>
								<p class="text-sm text-mist-300">Con đường {pathName(resolvePath(faction, draft.path) ?? 'tien')}</p>
							</div>
							<button type="button" class="btn btn-ghost h-10 shrink-0 px-3 text-sm" onclick={() => go('the-luc')}>Đổi</button>
						</div>
						<label class="block">
							<span class="text-sm text-mist-300">Đạo hiệu (2–24 chữ cái)</span>
							<!-- svelte-ignore a11y_autofocus -->
							<input
								bind:value={draft.name}
								required
								minlength="2"
								maxlength="24"
								autocomplete="off"
								autofocus
								class="mt-1 h-12 w-full rounded-[2px] border border-ink-600 bg-ink-950/70 px-3 text-lg text-mist-100 outline-none placeholder:text-mist-500 focus:border-gold-500"
								placeholder="Ví dụ: Lâm Thanh Vân"
							/>
						</label>
						<p class="text-xs text-mist-500">
							Không dùng từ tục tĩu, không giả danh ban quản trị, nhân vật cốt truyện hay nhân vật tín ngưỡng. Đạo hiệu vi phạm có thể
							bị đổi.
						</p>
						{#if error}<p role="alert" class="text-sm text-cinnabar-300">{error}</p>{/if}
						<button disabled={busy} class="btn btn-gold h-12 w-full">
							{busy ? 'Đang khắc tên…' : 'Khắc tên, vào Tử Tiêu Cung'}
						</button>
						{#if cooldownDays}
							<p class="text-center text-xs text-mist-500">Gia nhập xong phải chờ {cooldownDays} ngày mới đổi phe được.</p>
						{/if}
					</form>
				{/if}
			</main>
		{/key}
	</div>
{/if}

<style>
	.backdrop {
		position: fixed;
		inset: 0;
		z-index: var(--z-scene);
		background: var(--color-ink-950);
	}
	.backdrop img {
		width: 100%;
		height: 100%;
		object-fit: cover;
		object-position: 50% 20%;
		opacity: 0.28;
		filter: blur(2px);
	}
	.backdrop::after {
		content: '';
		position: absolute;
		inset: 0;
		background: linear-gradient(180deg, color-mix(in oklab, var(--color-ink-950) 40%, transparent), var(--color-ink-950) 85%);
	}
	nav {
		background: linear-gradient(180deg, var(--color-ink-950), color-mix(in oklab, var(--color-ink-950) 0%, transparent));
	}
	.dot {
		display: grid;
		place-items: center;
		width: 1.5rem;
		height: 1.5rem;
		flex-shrink: 0;
		border-radius: 50%;
		border: 1px solid var(--color-ink-600);
		color: var(--color-mist-500);
		font-size: 0.7rem;
	}
	.dot.is-done {
		border-color: var(--color-jade-500);
		color: var(--color-jade-300);
	}
	.dot.is-now {
		border-color: var(--color-gold-500);
		color: var(--color-gold-300);
	}
	.sage {
		position: relative;
		display: grid;
		place-items: center;
		width: min(70vw, 18rem);
		aspect-ratio: 1;
		margin-top: 0.5rem;
	}
	.sage img {
		position: relative;
		width: 100%;
		height: 100%;
		object-fit: contain;
	}
	.halo {
		position: absolute;
		inset: 8%;
		border-radius: 50%;
		background: radial-gradient(circle, color-mix(in oklab, var(--color-spirit-violet) 38%, transparent), transparent 68%);
		animation: var(--animate-qi-breathe);
	}
	.speech {
		margin-top: -1.5rem;
		position: relative;
		cursor: pointer;
	}
	.is-picked {
		border-color: var(--color-gold-500);
	}
	.chosen {
		border-color: color-mix(in oklab, var(--fc) 45%, transparent);
	}
</style>
