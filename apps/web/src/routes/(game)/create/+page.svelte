<script lang="ts">
	import { goto, invalidateAll } from '$app/navigation';
	import { api, ApiError } from '$lib/api/client';

	let name = $state('');
	let error = $state<string | null>(null);
	let busy = $state(false);

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = null;
		try {
			await api.createCharacter(name);
			await invalidateAll();
			await goto('/chon-the-luc');
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Không tạo được nhân vật';
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head><title>Tạo nhân vật · Tiên Đạo</title></svelte:head>

<form onsubmit={submit} class="panel mx-auto mt-10 max-w-md space-y-5 p-8">
	<div>
		<h1 class="text-4xl font-bold text-gold-300">Lập Đạo Hiệu</h1>
		<p class="mt-2 text-sm text-mist-300">
			Mỗi tài khoản có một nhân vật. Đạo hiệu sẽ được khắc trên Thiên Bảng — chọn kỹ nhé.
		</p>
		<p class="mt-2 text-xs text-mist-500">
			Không dùng từ tục tĩu, không giả danh ban quản trị, nhân vật cốt truyện hay nhân vật tín ngưỡng. Đạo hiệu vi phạm có thể bị đổi.
		</p>
	</div>
	<label class="block">
		<span class="text-sm text-mist-300">Đạo hiệu (2–24 chữ cái)</span>
		<input
			bind:value={name}
			required
			minlength="2"
			maxlength="24"
			autocomplete="off"
			class="mt-1 h-11 w-full rounded-[2px] border border-ink-600 bg-ink-950/60 px-3 text-mist-100 outline-none placeholder:text-mist-500 focus:border-gold-500"
			placeholder="Ví dụ: Lâm Thanh Vân"
		/>
	</label>
	{#if error}<p role="alert" class="text-sm text-cinnabar-300">{error}</p>{/if}
	<button
		disabled={busy}
		class="btn btn-jade h-12 w-full"
	>
		{busy ? 'Đang khai mở…' : 'Bước vào Tiên Đạo'}
	</button>
</form>
