<script>
	import { apiFetch, publicLink } from '$lib/api.js';
	import { formatSlot } from '$lib/time.js';
	import { onMount } from 'svelte';

	let surveys = [];
	let error = '';
	let loading = true;

	onMount(async () => {
		try {
			surveys = await apiFetch('/api/surveys');
		} catch (err) {
			error = 'Log in to create and manage scheduling surveys.';
		} finally {
			loading = false;
		}
	});
</script>

{#if loading}
	<p>Loading…</p>
{:else if error}
	<section class="hero">
		<h1>Find the time that works for everyone.</h1>
		<p>Create a survey, offer a few candidate time slots, and share one link so people can mark their availability.</p>
		<div><a class="button" href="/signup">Get started</a> <a class="button secondary" href="/login">Log in</a></div>
	</section>
{:else}
	<div class="grid">
		<div class="card two" style="align-items:center">
			<div>
				<h1>Your scheduling surveys</h1>
				<p>Share a survey link and collect availability.</p>
			</div>
			<p style="text-align:right"><a class="button" href="/surveys/new">Create survey</a></p>
		</div>
		<div class="survey-list">
			{#each surveys as survey}
				<article class="card">
					<h2>{survey.title}</h2>
					<p>{survey.description}</p>
					<ul class="slot-list">
						{#each survey.slots as slot}
							<li class="slot-pill">{formatSlot(slot)}</li>
						{/each}
					</ul>
					<p><a href={publicLink(survey.share_token)}>{publicLink(survey.share_token)}</a></p>
					<a class="button secondary" href={`/surveys/${survey.share_token}/results`}>View results</a>
				</article>
			{:else}
				<div class="card"><p>No surveys yet.</p></div>
			{/each}
		</div>
	</div>
{/if}
