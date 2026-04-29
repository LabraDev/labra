<script lang="ts">
	import type { Deployment } from '$lib/api';
	import { prettyDate } from '$lib/api';
	import StatusPill from '$lib/components/status-pill.svelte';
	import { prettyTrigger, sanitizeSiteURL, statusLabel, statusTone } from '$lib/deploy-status';

	export let deployments: Deployment[] = [];
	export let latestStatusSiteURL = '';
	export let latestSiteReachable = false;

	function deploymentStatusToneFor(dep: Deployment, depIndex: number): string {
		const isLatest = depIndex === 0;
		const siteURL = isLatest ? latestStatusSiteURL : sanitizeSiteURL(dep.site_url);
		const reachable = isLatest ? latestSiteReachable : undefined;
		return statusTone(dep.status, siteURL, reachable);
	}

	function deploymentStatusLabelFor(dep: Deployment, depIndex: number): string {
		const isLatest = depIndex === 0;
		const siteURL = isLatest ? latestStatusSiteURL : sanitizeSiteURL(dep.site_url);
		const reachable = isLatest ? latestSiteReachable : undefined;
		return statusLabel(dep.status, siteURL, reachable);
	}
</script>

<section class="config-history-section">
	{#if deployments.length === 0}
		<p class="muted">No deployments yet.</p>
	{:else}
		<table>
			<thead>
				<tr>
					<th class="config-history-title" colspan="5">Config History</th>
				</tr>
				<tr>
					<th class="config-history-colhead">ID</th>
					<th class="config-history-colhead">Status</th>
					<th class="config-history-colhead">Trigger</th>
					<th class="config-history-colhead">Updated</th>
					<th class="config-history-colhead">Details</th>
				</tr>
			</thead>
			<tbody>
				{#each deployments as dep, depIndex}
					<tr>
						<td>{dep.id}</td>
						<td>
							<StatusPill
								tone={deploymentStatusToneFor(dep, depIndex)}
								label={deploymentStatusLabelFor(dep, depIndex)}
							/>
						</td>
						<td>{prettyTrigger(dep.trigger_type)}</td>
						<td>{prettyDate(dep.updated_at)}</td>
						<td><a href={`/deploys/${dep.id}`}>open</a></td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}
</section>

<style>
	.config-history-title {
		text-align: center;
		font-size: 1.09rem;
		font-weight: 700;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		color: var(--text-color);
		padding: 0.72rem 0.7rem 0.56rem;
	}

	.config-history-section {
		margin: 0;
	}

	.config-history-section .muted {
		margin: 0;
	}

	.config-history-colhead {
		padding-top: 0.42rem;
	}
</style>
