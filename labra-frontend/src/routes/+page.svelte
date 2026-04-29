<script>
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import HeroSection from '$lib/components/hero.svelte';
	import GithubLoginButton from '$lib/components/githublogin.svelte';
	import { getSessionToken } from '$lib/api';

	// if theyre already logged in skip the landing page and go straight to dashboard
	onMount(() => {
		if (getSessionToken().trim().length > 0) {
			void goto('/dashboard');
		}
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
				<GithubLoginButton variant="large" />
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
		padding: 1.25rem 1.25rem 1.55rem;
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
		margin-top: 1.05rem;
	}

	.intro h2 {
		margin: 0 0 0.62rem;
	}

	.intro p {
		margin: 0;
		line-height: 1.48;
	}
</style>
