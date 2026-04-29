<script lang="ts">
	export let tone: string = 'neutral';
	export let label: string = '';
</script>

<span class={`status-pill ${tone}`}>{label}</span>

<style>
	.status-pill {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		padding: 0.2rem 0.55rem;
		border-radius: 999px;
		font-size: 0.78rem;
		font-weight: 700;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		border: 1px solid transparent;
	}

	.status-pill.ok {
		color: var(--green);
		border-color: rgba(166, 218, 149, 0.42);
		background: rgba(166, 218, 149, 0.14);
	}

	.status-pill.warn {
		color: var(--yellow);
		border-color: rgba(238, 212, 159, 0.42);
		background: rgba(238, 212, 159, 0.14);
	}

	.status-pill.error {
		color: var(--red);
		border-color: rgba(237, 135, 150, 0.42);
		background: rgba(237, 135, 150, 0.14);
	}

	.status-pill.busy {
		color: var(--blue);
		border-color: rgba(138, 173, 244, 0.44);
		background: rgba(138, 173, 244, 0.16);
	}

	.status-pill.neutral {
		color: var(--sky);
		border-color: rgba(145, 215, 227, 0.42);
		background: rgba(145, 215, 227, 0.14);
	}
</style>
