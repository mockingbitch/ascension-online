<!--
	Vân Khê in the onboarding wizard (/create): her lines for the current wizard step in the same visual-novel overlay
	as /play (GuideOverlay — big, on top; the wizard waits until she is done). No character exists yet, so progress is client-only (sessionStorage): "Đã hiểu" /
	"Bỏ qua" hide that step, "Không nhắc lại" hides the guide for the rest of the wizard.
-->
<script lang="ts">
	import { tutorial } from '$lib/game/tutorial.svelte';
	import { onboardingStep } from '$lib/game/tutorialRules';
	import GuideOverlay from './GuideOverlay.svelte';

	let { step }: { step: string } = $props();
	const KEY = 'tiendao:onboarding-guide';

	function read(): { hidden: string[]; off: boolean } {
		try {
			const v = JSON.parse(sessionStorage.getItem(KEY) ?? '') as { hidden?: string[]; off?: boolean };
			return { hidden: v.hidden ?? [], off: !!v.off };
		} catch {
			return { hidden: [], off: false };
		}
	}
	let seen = $state(read());
	const save = () => {
		try {
			sessionStorage.setItem(KEY, JSON.stringify(seen));
		} catch {
			/* not persisted: hidden for this page life */
		}
	};
	$effect(() => void tutorial.loadScript());

	const s = $derived(onboardingStep(tutorial.script, step));
	let i = $state(0);
	$effect(() => {
		void s?.id;
		i = 0;
	});
	const hide = () => {
		if (s) seen.hidden = [...seen.hidden, s.id];
		save();
	};
</script>

{#if s && !seen.off && !seen.hidden.includes(s.id)}
	<GuideOverlay
		name={tutorial.script?.name ?? 'Vân Khê'}
		step={s}
		{i}
		onboarding={s.id}
		onnext={() => (i + 1 < s.lines.length ? (i += 1) : hide())}
		onprev={() => (i = Math.max(0, i - 1))}
		onskip={hide}
		onmute={() => {
			seen.off = true;
			save();
		}}
	/>
{/if}
