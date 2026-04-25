<script>
	import { onMount } from 'svelte';
	import HeroSection from '$lib/components/hero.svelte';
	import GithubLoginButton from '$lib/components/githublogin.svelte';
	import { getSessionToken } from '$lib/api';

	let isAuthenticated = false;

	function syncAuthState() {
		isAuthenticated = getSessionToken().trim().length > 0;
	}

	onMount(() => {
		syncAuthState();
		window.addEventListener('storage', syncAuthState);
		return () => window.removeEventListener('storage', syncAuthState);
	});
</script>

<section class="home-shell">
	<HeroSection>
		<div class="intro">
			<h2>Build, deploy, and observe from one dashboard</h2>
			<p>
				Labra gives you live deployment status, log streams, and AI-assisted rollout guidance in
				one place.
			</p>
			<div class="actions">
				{#if isAuthenticated}
					<a class="button" href="/apps">Open App Dashboard</a>
					<a class="ghost" href="/dashboard">View Control Plane</a>
				{:else}
					<div class="login-stack">
						<GithubLoginButton variant="large" />
						<a class="session-signin" href="/login">Session Sign in</a>
					</div>
				{/if}
			</div>
		</div>
	</HeroSection>
</section>

<style>
	.home-shell {
		min-height: 100%;
	}

	.intro {
		width: min(900px, 100%);
		justify-self: center;
		text-align: center;
		padding: 1.2rem 1.2rem 1.5rem;
		background:
			linear-gradient(150deg, rgba(73, 77, 100, 0.3), rgba(36, 39, 58, 0.65) 42%, rgba(24, 25, 38, 0.88));
		border: 1px solid rgba(183, 189, 248, 0.25);
		border-radius: var(--radius-md);
		box-shadow: var(--shadow-soft);
		backdrop-filter: blur(7px);
	}

	.actions {
		display: flex;
		gap: 0.7rem;
		justify-content: center;
		flex-wrap: wrap;
		margin-top: 1rem;
	}

	.login-stack {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 0.55rem;
	}

	.session-signin {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		padding: 0.5rem 0.92rem;
		border-radius: 999px;
		border: 1px solid rgba(183, 189, 248, 0.34);
		background: rgba(30, 32, 48, 0.82);
		text-decoration: none;
		font-size: 0.88rem;
		font-weight: 600;
		color: var(--text-color);
		transition: transform 120ms ease, border-color 120ms ease, background-color 120ms ease;
	}

	.session-signin:hover {
		transform: translateY(-1px);
		border-color: rgba(139, 213, 202, 0.55);
		background: rgba(36, 39, 58, 0.95);
	}

	.ghost {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		padding: 0.55rem 0.95rem;
		border-radius: var(--radius-sm);
		border: 1px solid rgba(183, 189, 248, 0.32);
		background: rgba(30, 32, 48, 0.82);
		text-decoration: none;
		font-weight: 600;
	}
</style>
