<script>
	import { apiFetch } from '$lib/api.js';
	import { formatSlot } from '$lib/time.js';
	import { onMount } from 'svelte';

	export let data;
	let survey = null;
	let selected = new Set();
	let respondent_name = '';
	let error = '';
	let saved = false;

	onMount(async () => {
		try {
			survey = await apiFetch(`/api/public/surveys/${data.token}`);
		} catch (err) {
			error = err.message;
		}
	});

	function toggle(id) {
		if (selected.has(id)) selected.delete(id);
		else selected.add(id);
		selected = new Set(selected);
	}

	async function submit() {
		error = '';
		try {
			await apiFetch(`/api/public/surveys/${data.token}/responses`, {
				method: 'POST',
				body: JSON.stringify({ respondent_name, slot_ids: [...selected] })
			});
			saved = true;
		} catch (err) {
			error = err.message;
		}
	}
</script>

{#if error}<div class="error">{error}</div>{/if}
{#if !survey && !error}<p>Loading…</p>{/if}
{#if survey}
	<div class="card grid">
		<h1>{survey.title}</h1>
		<p>{survey.description}</p>
		{#if saved}
			<div class="success">Thanks — your availability was saved.</div>
		{:else}
			<label>Your name <input bind:value={respondent_name} /></label>
			<div class="grid">
				{#each survey.slots as slot}
					<button type="button" class:selected={selected.has(slot.id)} class="secondary" onclick={() => toggle(slot.id)}>
						{selected.has(slot.id) ? '✓ ' : ''}{formatSlot(slot)}
					</button>
				{/each}
			</div>
			<button onclick={submit}>Submit availability</button>
		{/if}
	</div>
{/if}
