<script lang="ts">
	import StatusPill from '$lib/components/status-pill.svelte';

	export let tone: string = 'neutral';
	export let label: string = '';
	export let progress: number = 0;
	export let awaitingURL: boolean = false;
	export let pulseOnce: boolean = false;
	export let pulseLoop: boolean = false;
	export let ariaPrefix: string = 'Deployment progress';
</script>

<div class="status-row">
	<StatusPill tone={tone} label={label} />
	<span class="muted">{progress}%</span>
</div>
<div
	class={`progress-track ${pulseOnce ? 'complete-once' : ''} ${pulseLoop ? 'complete-loop' : ''}`}
	role="img"
	aria-label={`${ariaPrefix} ${progress}%`}
>
	<div
		class={`progress-bar ${tone} ${pulseOnce ? 'complete-once' : ''} ${pulseLoop ? 'complete-loop' : ''}`}
		style={`width: ${progress}%;`}
	></div>
</div>
{#if awaitingURL}
	<p class="status-note">Finalizing deployment: waiting for a reachable CloudFront URL.</p>
{/if}

<style>
	.status-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 0.6rem;
		margin: 0.08rem 0 0.46rem;
	}

	.progress-track {
		width: 100%;
		height: 0.55rem;
		border-radius: 999px;
		background: rgba(183, 189, 248, 0.16);
		overflow: hidden;
		margin-bottom: 0.32rem;
		position: relative;
		isolation: isolate;
	}

	.progress-bar {
		height: 100%;
		border-radius: inherit;
		position: relative;
		overflow: hidden;
		transition: width 220ms ease;
	}

	.progress-bar.complete-once {
		animation: deploy-complete-pulse 4.8s ease-in-out 1;
	}

	.progress-bar.complete-once::after,
	.progress-bar.complete-loop::after {
		content: '';
		position: absolute;
		inset: 0;
		background: linear-gradient(
			115deg,
			rgba(255, 255, 255, 0) 20%,
			rgba(255, 255, 255, 0.5) 50%,
			rgba(255, 255, 255, 0) 80%
		);
		transform: translateX(-140%);
		pointer-events: none;
	}

	.progress-bar.complete-once::after {
		animation: deploy-complete-shine 4.2s ease-in-out 1;
	}

	.progress-bar.complete-loop {
		animation: deploy-complete-pulse 2.8s ease-in-out infinite;
	}

	.progress-bar.complete-loop::after {
		animation: deploy-complete-shine 2.4s ease-in-out infinite;
	}

	.progress-bar.ok {
		background: linear-gradient(120deg, rgba(166, 218, 149, 0.9), rgba(139, 213, 202, 0.9));
	}

	.progress-bar.warn {
		background: linear-gradient(120deg, rgba(238, 212, 159, 0.88), rgba(245, 169, 127, 0.88));
	}

	.progress-bar.error {
		background: linear-gradient(120deg, rgba(237, 135, 150, 0.9), rgba(245, 189, 230, 0.86));
	}

	.progress-bar.busy {
		background: linear-gradient(120deg, rgba(138, 173, 244, 0.9), rgba(183, 189, 248, 0.88));
	}

	.progress-bar.neutral {
		background: linear-gradient(120deg, rgba(145, 215, 227, 0.76), rgba(183, 189, 248, 0.78));
	}

	.status-note {
		margin: 0.1rem 0 0.4rem;
		padding: 0.45rem 0.55rem;
		border-radius: 0.5rem;
		font-size: 0.85rem;
		border: 1px solid rgba(238, 212, 159, 0.35);
		background: rgba(238, 212, 159, 0.1);
		color: var(--yellow);
	}

	@keyframes deploy-complete-pulse {
		0%,
		100% {
			filter: drop-shadow(0 0 0 rgba(166, 218, 149, 0));
		}
		50% {
			filter: drop-shadow(0 0 8px rgba(166, 218, 149, 0.28));
		}
	}

	@keyframes deploy-complete-shine {
		0% {
			transform: translateX(-140%);
			opacity: 0;
		}
		18% {
			opacity: 1;
		}
		58% {
			transform: translateX(140%);
			opacity: 0;
		}
		100% {
			transform: translateX(140%);
			opacity: 0;
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.progress-bar,
		.progress-bar.complete-once,
		.progress-bar.complete-once::after,
		.progress-bar.complete-loop,
		.progress-bar.complete-loop::after {
			animation: none !important;
		}
	}
</style>
