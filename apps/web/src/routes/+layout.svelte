<script>
	import '../app.css';
	import { apiFetch } from '$lib/api.js';
	import { onMount } from 'svelte';

	let user = null;
	let loading = true;

	onMount(async () => {
		try {
			user = await apiFetch('/api/me');
		} catch {
			user = null;
		} finally {
			loading = false;
		}
	});

	async function logout() {
		await apiFetch('/api/auth/logout', { method: 'POST', body: '{}' });
		user = null;
		window.location.href = '/login';
	}
</script>

<header class="topbar">
	<a class="brand" href="/">Scheduling</a>
	<nav>
		{#if loading}
			<span>Loading…</span>
		{:else if user}
			<a href="/surveys/new">New survey</a>
			<span>{user.name}</span>
			<button class="link" on:click={logout}>Logout</button>
		{:else}
			<a href="/login">Login</a>
			<a class="button" href="/signup">Sign up</a>
		{/if}
	</nav>
</header>

<main class="shell">
	<slot />
</main>
