<section class="hero" aria-label="Labra Hero">
	<div class="wash wash-a"></div>
	<div class="wash wash-b"></div>
	<div class="wash wash-c"></div>
	<div class="orb orb-a"></div>
	<div class="orb orb-b"></div>
	<div class="orb orb-c"></div>

	<div class="hero-inner">
		<div class="hero-content">
			<h1>LABRA</h1>
			<h2>Simplifying AWS-first delivery</h2>
		</div>

		<div class="hero-lower">
			<slot />
		</div>
	</div>
</section>

<style>
	.hero {
		position: relative;
		overflow: hidden;
		isolation: isolate;
		min-height: max(560px, calc(100dvh - var(--header-offset, 0px)));
		display: grid;
		place-items: center;
		border-radius: 0;
		border-bottom: 0;
		padding: clamp(2.4rem, 8vh, 4.6rem) 1rem clamp(2rem, 6vh, 4rem);
		background:
			radial-gradient(130% 150% at 12% 0%, rgba(198, 160, 246, 0.34), transparent 62%),
			radial-gradient(130% 145% at 100% 0%, rgba(138, 173, 244, 0.3), transparent 64%),
			linear-gradient(165deg, rgba(24, 25, 38, 0.95), rgba(30, 32, 48, 0.94), rgba(24, 25, 38, 0.98));
	}

	.hero::before {
		content: '';
		position: absolute;
		inset: -18%;
		z-index: 0;
		pointer-events: none;
		background:
			linear-gradient(120deg, transparent, rgba(183, 189, 248, 0.16), transparent 62%),
			linear-gradient(240deg, transparent, rgba(139, 213, 202, 0.14), transparent 58%);
		filter: blur(26px);
		animation: shimmer 7.5s ease-in-out infinite;
	}

	.hero::after {
		content: '';
		position: absolute;
		inset: 0;
		z-index: 1;
		pointer-events: none;
		background-image:
			linear-gradient(to right, rgba(202, 211, 245, 0.06) 1px, transparent 1px),
			linear-gradient(to bottom, rgba(202, 211, 245, 0.06) 1px, transparent 1px);
		background-size: 44px 44px;
		opacity: 0.2;
		mask-image: radial-gradient(circle at center, black 20%, transparent 86%);
	}

	.hero-inner {
		width: min(1120px, 100%);
		position: relative;
		text-align: center;
		z-index: 2;
		display: grid;
		gap: clamp(1.2rem, 3.5vh, 2rem);
	}

	.hero-content {
		padding: 0 1rem;
		animation: reveal 680ms cubic-bezier(0.18, 0.9, 0.2, 1.05);
	}

	.hero-lower {
		animation: reveal 760ms cubic-bezier(0.18, 0.9, 0.2, 1.05);
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
		filter: blur(5px);
		opacity: 0.78;
	}

	.orb-a {
		width: clamp(180px, 28vw, 310px);
		height: clamp(180px, 28vw, 310px);
		left: -5%;
		top: 12%;
		background: radial-gradient(circle, rgba(198, 160, 246, 0.54), transparent 72%);
		animation: drift-a 6.5s ease-in-out infinite;
	}

	.orb-b {
		width: clamp(180px, 24vw, 280px);
		height: clamp(180px, 24vw, 280px);
		right: -3%;
		bottom: -6%;
		background: radial-gradient(circle, rgba(139, 213, 202, 0.5), transparent 74%);
		animation: drift-b 7.8s ease-in-out infinite;
	}

	.orb-c {
		width: clamp(160px, 22vw, 260px);
		height: clamp(160px, 22vw, 260px);
		left: 48%;
		top: -9%;
		background: radial-gradient(circle, rgba(145, 215, 227, 0.35), transparent 74%);
		animation: drift-c 9.2s ease-in-out infinite;
	}

	.wash {
		position: absolute;
		inset: -18%;
		filter: blur(36px);
		opacity: 0.34;
		pointer-events: none;
	}

	.wash-a {
		background: conic-gradient(from 0deg at 30% 40%, rgba(198, 160, 246, 0.58), transparent 40%, rgba(138, 173, 244, 0.55), transparent 78%);
		animation: spin-wash 12s linear infinite;
	}

	.wash-b {
		background: conic-gradient(from 100deg at 65% 60%, rgba(139, 213, 202, 0.48), transparent 48%, rgba(145, 215, 227, 0.42), transparent 82%);
		animation: spin-wash-rev 14s linear infinite;
	}

	.wash-c {
		background: radial-gradient(circle at 50% 50%, rgba(183, 189, 248, 0.28), transparent 60%);
		animation: pulse-wash 5.6s ease-in-out infinite;
	}

	@keyframes reveal {
		0% { opacity: 0; transform: translateY(12px) scale(0.985); }
		100% { opacity: 1; transform: translateY(0) scale(1); }
	}

	@keyframes shimmer {
		0%, 100% { transform: translate3d(-2%, 1.5%, 0) scale(1); opacity: 0.35; }
		50% { transform: translate3d(2%, -1.5%, 0) scale(1.06); opacity: 0.5; }
	}

	@keyframes drift-a {
		0%, 100% { transform: translate(0, 0); }
		50% { transform: translate(24px, -16px); }
	}

	@keyframes drift-b {
		0%, 100% { transform: translate(0, 0); }
		50% { transform: translate(-20px, 14px); }
	}

	@keyframes drift-c {
		0%, 100% { transform: translate(0, 0); }
		50% { transform: translate(14px, 22px); }
	}

	@keyframes spin-wash {
		0% { transform: rotate(0deg) scale(1); }
		50% { transform: rotate(180deg) scale(1.05); }
		100% { transform: rotate(360deg) scale(1); }
	}

	@keyframes spin-wash-rev {
		0% { transform: rotate(360deg) scale(1); }
		50% { transform: rotate(180deg) scale(1.08); }
		100% { transform: rotate(0deg) scale(1); }
	}

	@keyframes pulse-wash {
		0%, 100% { opacity: 0.2; transform: scale(0.96); }
		50% { opacity: 0.42; transform: scale(1.04); }
	}

</style>
