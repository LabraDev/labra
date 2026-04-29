<script lang="ts">
	export let label: string = '';
	export let progress: number = 0;
	export let successNote: string = '';
	export let countdown: number = 0;
	export let completionFXActive: boolean = false;
</script>

<div class="delete-progress">
	<div class="status-row">
		<span class="muted">{label}</span>
		<span class="muted">{Math.round(progress)}%</span>
	</div>
	<div
		class={`progress-track ${completionFXActive ? 'complete-delete' : ''}`}
		role="img"
		aria-label={`Delete progress ${Math.round(progress)}%`}
	>
		<div
			class={`progress-bar delete ${completionFXActive ? 'complete-delete' : ''}`}
			style={`width: ${progress}%;`}
		></div>
	</div>
	{#if successNote}
		<p class="delete-success-note">{successNote}</p>
	{/if}
	{#if countdown > 0}
		<p class="muted">Redirecting in {countdown}s…</p>
	{/if}
</div>

<style>
	.delete-progress {
		display: grid;
		gap: 0.36rem;
		margin-top: 0.2rem;
	}

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

	.progress-track.complete-delete {
		animation: delete-shockwave 2.6s ease-out 1;
	}

	.progress-bar {
		height: 100%;
		border-radius: inherit;
		position: relative;
		overflow: hidden;
		transition: width 220ms ease;
	}

	.progress-bar.delete {
		background: linear-gradient(120deg, rgba(245, 169, 127, 0.92), rgba(237, 135, 150, 0.9));
	}

	.progress-bar.complete-delete {
		animation: delete-complete-pulse 2.4s ease-in-out 1;
	}

	.progress-bar.complete-delete::after {
		content: '';
		position: absolute;
		inset: 0;
		background: linear-gradient(
			112deg,
			rgba(255, 255, 255, 0) 15%,
			rgba(255, 247, 219, 0.58) 45%,
			rgba(255, 255, 255, 0) 78%
		);
		transform: translateX(-140%);
		animation: delete-complete-shine 2.8s ease-in-out 1;
		pointer-events: none;
	}

	.delete-success-note {
		margin: 0.1rem 0 0.18rem;
		color: #a6da95;
		font-size: 0.84rem;
		font-weight: 600;
	}

	@keyframes delete-complete-pulse {
		0%,
		100% {
			filter: saturate(100%) brightness(1);
		}
		50% {
			filter: saturate(124%) brightness(1.12);
		}
	}

	@keyframes delete-complete-shine {
		0% {
			transform: translateX(-140%);
			opacity: 0;
		}
		15% {
			opacity: 1;
		}
		52% {
			transform: translateX(140%);
			opacity: 0;
		}
		100% {
			transform: translateX(140%);
			opacity: 0;
		}
	}

	@keyframes delete-shockwave {
		0%,
		100% {
			box-shadow: 0 0 0 0 rgba(245, 169, 127, 0);
		}
		52% {
			box-shadow: 0 0 0 5px rgba(245, 169, 127, 0.24);
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.progress-track.complete-delete,
		.progress-bar.complete-delete,
		.progress-bar.complete-delete::after {
			animation: none !important;
		}
	}
</style>
