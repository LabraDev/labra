<script>
	import HeroSection from '$lib/components/hero.svelte';
</script>

<HeroSection />

<section class="page">
	<div class="intro card">
		<h2>Build, deploy, and observe from one dashboard</h2>
		<p>
			Labra turns a Git push into cloud infrastructure updates with deployment status,
			logs, AI-assisted insights, and customer account rollout support.
		</p>
		<div class="actions">
			<a class="button" href="/apps">Open App Dashboard</a>
			<a class="ghost" href="/dashboard">View Control Plane</a>
		</div>
	</div>
</section>

<style>
	.intro {
		max-width: 820px;
		justify-self: center;
		text-align: center;
		padding: 1.2rem 1.2rem 1.35rem;
	}

	.actions {
		display: flex;
		gap: 0.7rem;
		justify-content: center;
		flex-wrap: wrap;
		margin-top: 0.9rem;
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
