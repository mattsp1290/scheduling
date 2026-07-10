<script>
	import { apiFetch } from '$lib/api.js';
	let email = '';
	let password = '';
	let error = '';

	async function login() {
		error = '';
		try {
			await apiFetch('/api/auth/login', { method: 'POST', body: JSON.stringify({ email, password }) });
			window.location.href = '/';
		} catch (err) {
			error = err.message;
		}
	}
</script>

<div class="card" style="max-width:520px;margin:auto">
	<h1>Log in</h1>
	<div class="grid">
		{#if error}<div class="error">{error}</div>{/if}
		<label>Email <input bind:value={email} type="email" autocomplete="email" /></label>
		<label>Password <input bind:value={password} type="password" autocomplete="current-password" /></label>
		<button onclick={login}>Log in</button>
		<p>Need an account? <a href="/signup">Sign up</a>.</p>
	</div>
</div>
