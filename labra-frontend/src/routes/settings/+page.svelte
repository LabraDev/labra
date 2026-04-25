<script lang="ts">
	import { onMount } from 'svelte';
	import { apiDELETE, apiGET, apiPOST } from '$lib/api';

	type AWSConnection = {
		id: number;
		role_arn: string;
		external_id: string;
		region: string;
		account_id: string;
		status: string;
		updated_at: number;
	};
	type AWSAccessTab = 'connection' | 'connected';

	const platformPrincipalPlaceholder = 'arn:aws:iam::<LABRA_ACCOUNT_ID>:role/<LABRA_PLATFORM_ROLE>';
	const externalIdPlaceholder = '<LABRA_EXTERNAL_ID>';
	const usRegions = ['us-east-1', 'us-east-2', 'us-west-1', 'us-west-2'];
	const defaultCloudFormationTemplateS3URL =
		'https://labra-infra-dev-platform-974646089985-site.s3.us-west-1.amazonaws.com/customer-assume-role.cfn.yaml';
	const configuredCloudFormationTemplateS3URL = (import.meta.env.VITE_CLOUDFORMATION_TEMPLATE_S3_URL ?? '').trim();

	let roleARN = '';
	let externalID = '';
	let region = 'us-west-2';
	let loading = false;
	let error = '';
	let success = '';
	let connections: AWSConnection[] = [];
	let templateURL = '';
	let quickCreateTemplateURL = configuredCloudFormationTemplateS3URL || defaultCloudFormationTemplateS3URL;
	let quickCreateURL = '';
	let activeTab: AWSAccessTab = 'connection';

	$: quickCreateURL = quickCreateTemplateURL
		? `https://console.aws.amazon.com/cloudformation/home?region=${encodeURIComponent(region)}#/stacks/quickcreate?templateURL=${encodeURIComponent(quickCreateTemplateURL)}&stackName=labra-customer-assume-role`
		: '';

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

	async function disconnectConnection(connectionID: number) {
		error = '';
		success = '';
		loading = true;
		try {
			await apiDELETE(`/v1/aws-connections/${connectionID}`);
			success = 'AWS connection removed.';
			await refreshConnections();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to disconnect AWS connection';
		} finally {
			loading = false;
		}
	}

	onMount(async () => {
		if (typeof window !== 'undefined') {
			templateURL = `${window.location.origin}/customer-assume-role.cfn.yaml`;
		}

		try {
			await refreshConnections();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load AWS connections';
		}
	});
</script>

<section class="page">
	<h1>AWS Access</h1>

	<div class="tabs" role="tablist" aria-label="AWS access sections">
		<button
			type="button"
			role="tab"
			class="tab-button"
			class:active={activeTab === 'connection'}
			aria-selected={activeTab === 'connection'}
			on:click={() => (activeTab = 'connection')}
		>
			Connection
		</button>
		<button
			type="button"
			role="tab"
			class="tab-button"
			class:active={activeTab === 'connected'}
			aria-selected={activeTab === 'connected'}
			on:click={() => (activeTab = 'connected')}
		>
			Connected Accounts
		</button>
	</div>

	{#if activeTab === 'connection'}
		<div class="card" role="tabpanel">
			<h2>1. Create AssumeRole (One-Click)</h2>
			<p class="muted">
				Use the CloudFormation template below in your AWS account. This is the intended one-click flow.
			</p>
			<div class="actions">
				<a class="button" href={quickCreateURL} target="_blank" rel="noreferrer">
					Open CloudFormation Quick Create
				</a>
				<a class="ghost" href={templateURL} target="_blank" rel="noreferrer">
					View Template YAML
				</a>
			</div>
			<p class="muted">
				In the stack parameters, set:
			</p>
			<ul class="steps">
				<li><code>PlatformPrincipalArn</code>: {platformPrincipalPlaceholder}</li>
				<li><code>ExternalId</code>: {externalIdPlaceholder}</li>
				<li><code>RoleName</code>: keep default unless you need a custom name</li>
			</ul>
			<p class="muted">
				After stack creation, copy the <code>CustomerRoleArn</code> value from Outputs.
			</p>
		</div>

		<div class="card" role="tabpanel">
			<h2>2. Save Connection in Labra</h2>
			<div class="aws-form-grid">
				<label for="role-arn">Role ARN</label>
				<input id="role-arn" bind:value={roleARN} placeholder="arn:aws:iam::123456789012:role/labra-access" />
				<label for="external-id">External ID</label>
				<input id="external-id" bind:value={externalID} placeholder="external-id-123" />
				<label for="region">Region</label>
				<select id="region" bind:value={region}>
					{#each usRegions as regionOption}
						<option value={regionOption}>{regionOption}</option>
					{/each}
				</select>
			</div>
			<div class="form-actions">
				<button on:click={connectAWS} disabled={loading}> {loading ? 'Saving...' : 'Validate + Save'} </button>
			</div>
		</div>
	{:else}
		<div class="card" role="tabpanel">
			<h2>Connected AWS Accounts</h2>
			{#if connections.length === 0}
				<p class="muted">No connections yet.</p>
			{:else}
				<ul class="connection-list">
					{#each connections as c}
						<li>
							<div class="connection-row">
								<strong>{c.account_id}</strong> - {c.region} - {c.status}
								<div class="small">{c.role_arn}</div>
								<button
									type="button"
									class="disconnect-button"
									on:click={() => disconnectConnection(c.id)}
									disabled={loading}
								>
									Disconnect
								</button>
							</div>
						</li>
					{/each}
				</ul>
			{/if}
		</div>
	{/if}

	{#if error}<p class="error">{error}</p>{/if}
	{#if success}<p class="success">{success}</p>{/if}
</section>

<style>
	.tabs {
		display: flex;
		gap: 0.5rem;
		flex-wrap: wrap;
	}

	.tab-button {
		background: rgba(30, 32, 48, 0.82);
		color: var(--text-color, #cad3f5);
		border: 1px solid rgba(183, 189, 248, 0.32);
		box-shadow: none;
	}

	.tab-button.active {
		background: linear-gradient(130deg, var(--blue, #8aadf4), var(--lavender, #b7bdf8));
		color: var(--crust, #181926);
		border-color: rgba(138, 173, 244, 0.6);
	}

	.actions {
		display: flex;
		flex-wrap: wrap;
		gap: 0.7rem;
		margin: 0.6rem 0 0.75rem;
	}

	.actions .button {
		text-decoration: none;
	}

	.ghost {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		padding: 0.58rem 0.9rem;
		border-radius: var(--radius-sm);
		border: 1px solid rgba(183, 189, 248, 0.32);
		background: rgba(30, 32, 48, 0.82);
		text-decoration: none;
		font-weight: 600;
	}

	.steps {
		display: grid;
		gap: 0.38rem;
		margin: 0.35rem 0 0.5rem;
		padding-left: 1.2rem;
	}

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

	.connection-row {
		display: grid;
		gap: 0.35rem;
	}

	.disconnect-button {
		width: fit-content;
		background: linear-gradient(130deg, var(--red, #ed8796), var(--maroon, #ee99a0));
		border-color: rgba(237, 135, 150, 0.65);
	}

	.form-actions {
		display: flex;
		margin-top: 0.75rem;
	}

	.form-actions button {
		width: fit-content;
	}

	.aws-form-grid {
		display: grid;
		gap: 0.5rem;
	}

	.aws-form-grid label {
		margin-top: 0.25rem;
	}

	.small {
		margin-top: 0.2rem;
		font-size: 0.82rem;
		opacity: 0.78;
	}
</style>
