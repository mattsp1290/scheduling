<script>
	import { apiFetch, publicLink } from '$lib/api.js';
	import { formatSlot } from '$lib/time.js';
	import { onMount } from 'svelte';

	export let data;
	let results = null;
	let error = '';

	onMount(async () => {
		try {
			results = await apiFetch(`/api/surveys/${data.token}/results`);
		} catch (err) {
			error = err.message;
		}
	});
</script>

{#if error}<div class="error">{error}</div>{/if}
{#if !results && !error}<p>Loading…</p>{/if}
{#if results}
	<div class="grid">
		<div class="card">
			<h1>{results.survey.title} results</h1>
			<p>Share link: <a href={publicLink(results.survey.share_token)}>{publicLink(results.survey.share_token)}</a></p>
		</div>
		<div class="card">
			<h2>Availability by slot</h2>
			{#each results.survey.slots as slot}
				<div class="result-row">
					<div>
						<strong>{formatSlot(slot)}</strong>
						<p>{(results.respondents[slot.id] || []).join(', ') || 'No one yet'}</p>
					</div>
					<strong>{results.slot_counts[slot.id] || 0} available</strong>
				</div>
			{/each}
		</div>
		<div class="card">
			<h2>Responses</h2>
			{#each results.responses as response}
				<p><strong>{response.respondent_name}</strong> selected {response.slot_ids.length} slot{response.slot_ids.length === 1 ? '' : 's'}.</p>
			{:else}
				<p>No responses yet.</p>
			{/each}
		</div>
	</div>
{/if}
