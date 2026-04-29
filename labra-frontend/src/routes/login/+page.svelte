<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { onMount } from 'svelte';
	import GithubLoginButton from '$lib/components/githublogin.svelte';
	import { consumePostLoginRedirect, setSessionToken } from '$lib/api';

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
		success = 'Signed in successfully.';
		const postLoginTarget = consumePostLoginRedirect('/dashboard');

		if (window.history?.replaceState) {
			window.history.replaceState({}, '', '/login');
		}

		setTimeout(() => {
			void goto(postLoginTarget);
		}, 250);
	});
</script>

<section class="page">
	<div class="card auth-card">
		<h1>Sign In</h1>
		{#if notice}<p class="notice">{notice}</p>{/if}
		<p class="muted">
			Sign in with GitHub to continue.
		</p>
		<p class="muted">
			You will be redirected back here after authentication.
		</p>
		<div class="actions">
			<GithubLoginButton variant="large" postLoginPath="/dashboard" />
		</div>
		{#if success}<p class="success">{success}</p>{/if}
	</div>
</section>

<style>
	.auth-card {
		max-width: 760px;
		justify-self: center;
		display: grid;
		gap: 0.9rem;
	}

	.auth-card :global(p) {
		margin: 0;
	}

	.actions {
		display: flex;
		justify-content: center;
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
