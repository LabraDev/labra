<footer class="shell-footer">
	<div class="footer-inner">
		<p class="brand">Labra Co.</p>
		<div class="links" aria-label="Footer Links">
			<a href="/">About</a>
			<a href="/settings">Contact</a>
		</div>
		<p class="meta">Built for cloud-native demo deployments.</p>
	</div>
</footer>

<style>
	.shell-footer {
		border-top: 1px solid rgba(183, 189, 248, 0.18);
		background: linear-gradient(180deg, rgba(30, 32, 48, 0.9), rgba(24, 25, 38, 0.98));
		padding: 1rem 1rem 1.4rem;
	}

	.footer-inner {
		max-width: 1100px;
		margin: 0 auto;
		display: grid;
		gap: 0.5rem;
		justify-items: center;
	}

	.brand {
		font-family: 'Sora', sans-serif;
		font-weight: 700;
		letter-spacing: 0.06em;
		color: var(--rosewater);
	}

	.links {
		display: flex;
		gap: 0.9rem;
	}

	.links a {
		font-size: 0.9rem;
		text-decoration: none;
		color: var(--lavender);
	}

	.meta {
		font-size: 0.82rem;
		opacity: 0.72;
	}
</style>
