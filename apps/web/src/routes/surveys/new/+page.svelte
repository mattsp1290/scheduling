<script>
	import CalendarSlotPicker from '$lib/CalendarSlotPicker.svelte';
	import { apiFetch, publicLink } from '$lib/api.js';

	let title = '';
	let description = '';
	let timezone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
	let slots = [];
	let error = '';
	let created = null;

	async function createSurvey() {
		error = '';
		created = null;
		try {
			created = await apiFetch('/api/surveys', {
				method: 'POST',
				body: JSON.stringify({ title, description, timezone, slots })
			});
		} catch (err) {
			error = err.message;
		}
	}
</script>

<div class="card grid">
	<h1>Create a scheduling survey</h1>
	{#if error}<div class="error">{error}</div>{/if}
	{#if created}
		<div class="success">
			<strong>Survey created.</strong>
			<p>Share this link: <a href={publicLink(created.share_token)}>{publicLink(created.share_token)}</a></p>
			<a class="button secondary" href={`/surveys/${created.share_token}/results`}>View results</a>
		</div>
	{/if}
	<div class="two">
		<label>Title <input bind:value={title} placeholder="Team planning" /></label>
		<label>Timezone <input bind:value={timezone} /></label>
	</div>
	<label>Description <textarea bind:value={description} placeholder="Choose every slot that works for you."></textarea></label>
	<h2>Candidate time slots</h2>
	<CalendarSlotPicker bind:slots />
	<button onclick={createSurvey}>Create share link</button>
</div>
