<script lang="ts">
	import { createAuthSession } from '$lib/api';

	let externalJWT = '';
	let loading = false;
	let error = '';
	let success = '';

	async function handleLogin() {
		error = '';
		success = '';
		loading = true;
		try {
			const session = await createAuthSession(externalJWT.trim());
			success = `Signed in as ${session.principal.email ?? session.principal.sub}`;
			setTimeout(() => {
				window.location.href = '/dashboard';
			}, 350);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Login failed';
		} finally {
			loading = false;
		}
	}
</script>

<section class="page">
	<div class="card auth-card">
		<h1>Login</h1>
		<p class="muted">Paste your Cognito JWT for Sprint 2 auth bootstrap.</p>

		<label for="jwt">Cognito JWT</label>
		<textarea id="jwt" bind:value={externalJWT} rows="8" placeholder="eyJhbGciOi..."></textarea>
		<button on:click={handleLogin} disabled={loading || externalJWT.trim().length === 0}>
			{loading ? 'Signing in...' : 'Create Session'}
		</button>

		{#if error}<p class="error">{error}</p>{/if}
		{#if success}<p class="success">{success}</p>{/if}
	</div>
</section>

<style>
	.auth-card {
		max-width: 760px;
		justify-self: center;
		display: grid;
		gap: 0.8rem;
	}
</style>
