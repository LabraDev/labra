<section class="hero" aria-label="Labra Hero">
	<div class="orb orb-a"></div>
	<div class="orb orb-b"></div>
	<div class="hero-content">
		<p class="eyebrow">Ship from Git push to cloud URL</p>
		<h1>LABRA</h1>
		<h2>Mini Vercel for AWS-first delivery</h2>
	</div>
</section>

<style>
	.hero {
		position: relative;
		overflow: hidden;
		min-height: clamp(290px, 46vh, 480px);
		display: grid;
		place-items: center;
		border-radius: 0 0 26px 26px;
		border-bottom: 1px solid rgba(183, 189, 248, 0.24);
		background:
			radial-gradient(80% 140% at 4% 0%, rgba(198, 160, 246, 0.36), transparent 60%),
			radial-gradient(90% 140% at 100% 5%, rgba(138, 173, 244, 0.3), transparent 62%),
			linear-gradient(160deg, rgba(24, 25, 38, 0.95), rgba(30, 32, 48, 0.94), rgba(24, 25, 38, 0.98));
	}

	.hero-content {
		text-align: center;
		z-index: 2;
		padding: 0 1rem;
		animation: reveal 680ms cubic-bezier(0.18, 0.9, 0.2, 1.05);
	}

	.eyebrow {
		font-size: 0.88rem;
		letter-spacing: 0.13em;
		text-transform: uppercase;
		color: var(--teal);
		margin-bottom: 0.5rem;
	}

	h1 {
		font-size: clamp(2.35rem, 8vw, 5.5rem);
		line-height: 0.95;
		letter-spacing: 0.08em;
		margin: 0;
		color: transparent;
		background: linear-gradient(130deg, var(--rosewater), var(--lavender), var(--sky));
		-webkit-background-clip: text;
		background-clip: text;
	}

	h2 {
		margin-top: 0.9rem;
		font-size: clamp(1.03rem, 2.2vw, 1.45rem);
		font-weight: 500;
		color: var(--subtext);
	}

	.orb {
		position: absolute;
		border-radius: 999px;
		filter: blur(3px);
		opacity: 0.8;
	}

	.orb-a {
		width: clamp(180px, 28vw, 310px);
		height: clamp(180px, 28vw, 310px);
		left: -4%;
		top: 8%;
		background: radial-gradient(circle, rgba(198, 160, 246, 0.5), transparent 72%);
		animation: drift-a 9s ease-in-out infinite;
	}

	.orb-b {
		width: clamp(180px, 24vw, 280px);
		height: clamp(180px, 24vw, 280px);
		right: -2%;
		bottom: -6%;
		background: radial-gradient(circle, rgba(139, 213, 202, 0.44), transparent 74%);
		animation: drift-b 11s ease-in-out infinite;
	}

	@keyframes reveal {
		0% { opacity: 0; transform: translateY(12px) scale(0.985); }
		100% { opacity: 1; transform: translateY(0) scale(1); }
	}

	@keyframes drift-a {
		0%, 100% { transform: translate(0, 0); }
		50% { transform: translate(18px, -12px); }
	}

	@keyframes drift-b {
		0%, 100% { transform: translate(0, 0); }
		50% { transform: translate(-15px, 10px); }
	}
</style>
