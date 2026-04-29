<script lang="ts">
	import DeleteProgressPanel from '$lib/components/delete-progress-panel.svelte';

	export let showDeleteConfirm = false;
	export let deletePhrase = '';
	export let deleteConfirmInput = '';
	export let actionBusy = false;
	export let loading = false;
	export let deletePhraseMatches = false;
	export let deleteInProgress = false;
	export let deleteProgressLabel = '';
	export let deleteProgress = 0;
	export let deleteSuccessNotice = '';
	export let deleteRedirectCountdown = 0;
	export let deleteCompletionFXActive = false;
	export let onToggle: () => void = () => {};
	export let onDelete: () => void = () => {};
</script>

<section class="danger-zone">
	<h2>Danger Zone</h2>
	<p class="muted danger-copy">Deleting this app permanently removes it from Labra.</p>
	<button class="danger danger-toggle" on:click={onToggle} disabled={actionBusy || loading}>
		{showDeleteConfirm ? 'Cancel' : 'Delete App'}
	</button>

	{#if showDeleteConfirm}
		<div class="delete-confirm">
			<label for="delete-confirm-input">
				Type <code>{deletePhrase}</code> to confirm
			</label>
			<input
				id="delete-confirm-input"
				type="text"
				bind:value={deleteConfirmInput}
				placeholder={deletePhrase}
				autocomplete="off"
				spellcheck="false"
			/>
			<button
				class="danger"
				on:click={onDelete}
				disabled={actionBusy || loading || !deletePhraseMatches}
			>
				{actionBusy ? 'Deleting...' : 'Permanently Delete App'}
			</button>
			{#if deleteInProgress}
				<DeleteProgressPanel
					label={deleteProgressLabel}
					progress={deleteProgress}
					successNote={deleteSuccessNotice}
					countdown={deleteRedirectCountdown}
					completionFXActive={deleteCompletionFXActive}
				/>
			{/if}
		</div>
	{/if}
</section>

<style>
	.danger-zone {
		margin-top: 1rem;
		padding: 0.82rem 1rem 1rem;
		border-radius: 0.75rem;
		border: 1px solid rgba(237, 135, 150, 0.35);
		background: rgba(237, 135, 150, 0.07);
	}

	.danger-zone h2 {
		margin: 0 0 0.25rem;
	}

	.danger-copy {
		margin: 0.05rem 0 0.45rem;
	}

	.danger-toggle {
		margin-top: 0.35rem;
		font-weight: 700;
		letter-spacing: 0.01em;
		color: #ffe8ec;
		background: linear-gradient(135deg, rgba(237, 135, 150, 0.42), rgba(237, 135, 150, 0.3));
		border-color: rgba(237, 135, 150, 0.78);
		box-shadow: 0 8px 18px rgba(237, 135, 150, 0.24);
	}

	.danger-toggle:hover {
		color: #fff5f7;
		background: linear-gradient(135deg, rgba(237, 135, 150, 0.54), rgba(237, 135, 150, 0.38));
		border-color: rgba(237, 135, 150, 0.9);
		box-shadow: 0 10px 22px rgba(237, 135, 150, 0.28);
	}

	.danger-toggle:focus-visible {
		outline: 2px solid rgba(237, 135, 150, 0.9);
		outline-offset: 2px;
	}

	.delete-confirm {
		display: grid;
		gap: 0.56rem;
		margin-top: 0.72rem;
	}

	.delete-confirm label {
		font-size: 0.86rem;
	}

	.danger {
		width: fit-content;
		color: #f5b7c0;
		background: rgba(237, 135, 150, 0.14);
		border-color: rgba(237, 135, 150, 0.42);
		box-shadow: none;
		transition:
			background-color 130ms ease,
			border-color 130ms ease,
			color 130ms ease;
	}

	.danger:hover {
		transform: none;
		filter: none;
		box-shadow: none;
		background: rgba(237, 135, 150, 0.2);
		border-color: rgba(237, 135, 150, 0.56);
		color: #ffd6dc;
	}
</style>
