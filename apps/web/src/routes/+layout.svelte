<script lang="ts">
	import '../app.css';
	import WorldBackdrop from '$lib/components/WorldBackdrop.svelte';

	let { children } = $props();
</script>

<WorldBackdrop />
{@render children()}
