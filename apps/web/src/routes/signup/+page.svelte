<script>
	import { apiFetch } from '$lib/api.js';
	let name = '';
	let email = '';
	let password = '';
	let error = '';

	async function signup() {
		error = '';
		try {
			await apiFetch('/api/auth/signup', { method: 'POST', body: JSON.stringify({ name, email, password }) });
			window.location.href = '/';
		} catch (err) {
			error = err.message;
		}
	}
</script>

<div class="card" style="max-width:520px;margin:auto">
	<h1>Create an account</h1>
	<div class="grid">
		{#if error}<div class="error">{error}</div>{/if}
		<label>Name <input bind:value={name} autocomplete="name" /></label>
		<label>Email <input bind:value={email} type="email" autocomplete="email" /></label>
		<label>Password <input bind:value={password} type="password" autocomplete="new-password" /></label>
		<button onclick={signup}>Sign up</button>
		<p>Already have an account? <a href="/login">Log in</a>.</p>
	</div>
</div>
