<script lang="ts">
	import { onMount } from 'svelte';
	import { apiGET, apiPOST } from '$lib/api';

	type AWSConnection = {
		id: number;
		role_arn: string;
		external_id: string;
		region: string;
		account_id: string;
		status: string;
		updated_at: number;
	};

	let roleARN = '';
	let externalID = '';
	let region = 'us-west-2';
	let loading = false;
	let error = '';
	let success = '';
	let connections: AWSConnection[] = [];

	async function refreshConnections() {
		const result = await apiGET<{ aws_connections: AWSConnection[] }>('/v1/aws-connections');
		connections = result.aws_connections;
	}

	async function connectAWS() {
		error = '';
		success = '';
		loading = true;
		try {
			await apiPOST('/v1/aws-connections', {
				role_arn: roleARN,
				external_id: externalID,
				region
			});
			success = 'AWS connection validated and saved.';
			await refreshConnections();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to connect AWS';
		} finally {
			loading = false;
		}
	}

	onMount(async () => {
		try {
			await refreshConnections();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load AWS connections';
		}
	});
</script>

<section class="page">
	<h1>Settings</h1>
	<p class="muted">Connect your AWS account using AssumeRole metadata.</p>

	<div class="card">
		<h2>Connect AWS</h2>
		<label for="role-arn">Role ARN</label>
		<input id="role-arn" bind:value={roleARN} placeholder="arn:aws:iam::123456789012:role/labra-access" />
		<label for="external-id">External ID</label>
		<input id="external-id" bind:value={externalID} placeholder="external-id-123" />
		<label for="region">Region</label>
		<input id="region" bind:value={region} placeholder="us-west-2" />
		<button on:click={connectAWS} disabled={loading}> {loading ? 'Saving...' : 'Validate + Save'} </button>
	</div>

	{#if error}<p class="error">{error}</p>{/if}
	{#if success}<p class="success">{success}</p>{/if}

	<div class="card">
		<h2>Existing Connections</h2>
		{#if connections.length === 0}
			<p class="muted">No connections yet.</p>
		{:else}
			<ul class="connection-list">
				{#each connections as c}
					<li>
						<div>
							<strong>{c.account_id}</strong> - {c.region} - {c.status}
							<div class="small">{c.role_arn}</div>
						</div>
					</li>
				{/each}
			</ul>
		{/if}
	</div>
</section>

<style>
	.connection-list {
		list-style: none;
		display: grid;
		gap: 0.6rem;
		padding: 0;
		margin: 0;
	}

	.connection-list li {
		padding: 0.7rem 0.8rem;
		border: 1px solid rgba(183, 189, 248, 0.2);
		border-radius: var(--radius-sm);
		background: rgba(24, 25, 38, 0.45);
	}

	.small {
		margin-top: 0.2rem;
		font-size: 0.82rem;
		opacity: 0.78;
	}
</style>
