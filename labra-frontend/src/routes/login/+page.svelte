<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { onMount } from 'svelte';
	import { consumePostLoginRedirect, createAuthSession, setSessionToken } from '$lib/api';

	let externalJWT = '';
	let loading = false;
	let error = '';
	let success = '';
	let notice = '';

	$: notice = $page.url.searchParams.get('notice')?.trim() ?? '';

	onMount(() => {
		const hash = window.location.hash?.trim() ?? '';
		if (!hash.startsWith('#') || hash.length <= 1) return;

		const params = new URLSearchParams(hash.slice(1));
		const sessionToken = params.get('session_token')?.trim() ?? '';
		if (sessionToken.length === 0) return;

		setSessionToken(sessionToken);
		success = 'Signed in with GitHub.';
		error = '';
		const postLoginTarget = consumePostLoginRedirect('/dashboard');

		if (window.history?.replaceState) {
			window.history.replaceState({}, '', '/login');
		}

		setTimeout(() => {
			void goto(postLoginTarget);
		}, 250);
	});

	async function handleLogin() {
		error = '';
		success = '';
		loading = true;
		try {
			const session = await createAuthSession(externalJWT.trim());
			success = `Signed in as ${session.principal.email ?? session.principal.sub}`;
			const postLoginTarget = consumePostLoginRedirect('/dashboard');
			setTimeout(() => {
				void goto(postLoginTarget);
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
		<h1>Session Sign-In</h1>
		{#if notice}<p class="notice">{notice}</p>{/if}
		<p class="muted">
			Secondary option: exchange a Cognito JWT for a Labra session token.
		</p>
		<p class="muted">
			Use this when GitHub sign-in is unavailable. After creating the session, protected tabs unlock.
		</p>

		<label for="jwt">Cognito JWT (ID/Access token)</label>
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

	.notice {
		margin: 0;
		padding: 0.6rem 0.75rem;
		border-radius: var(--radius-sm);
		border: 1px solid rgba(245, 169, 127, 0.45);
		background: rgba(36, 39, 58, 0.82);
		color: var(--peach);
	}
</style>
