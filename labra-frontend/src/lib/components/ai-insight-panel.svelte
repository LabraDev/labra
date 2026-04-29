<script lang="ts">
	import type { AIDeployInsightResponse, AIRequestLog } from '$lib/api';
	import { apiGET, apiPOST, prettyDate } from '$lib/api';
	import {
		historyInsightPreview,
		insightTextFromHistory,
		renderInsightMarkdown
	} from '$lib/ai-insight';

export let deployID: number | null = null;
export let disabled = false;
export let title = 'AI Insight';
export let generateLabel = 'Generate AI Insight';
export let historyLabel = 'Recent AI Requests';

	let aiPrompt = '';
	let aiBusy = false;
	let aiError = '';
	let aiResult: AIDeployInsightResponse | null = null;
	let aiResultHTML = '';
	let aiHistory: AIRequestLog[] = [];
	let selectedAIHistoryID: number | null = null;
	let loadedDeployID: number | null = null;

	$: aiResultHTML = aiResult ? renderInsightMarkdown(aiResult.insight) : '';
	$: if (deployID && deployID !== loadedDeployID) {
		void loadAIHistory(deployID);
	}

	async function loadAIHistory(targetDeployID: number) {
		loadedDeployID = targetDeployID;
		try {
			const res = await apiGET<{ logs: AIRequestLog[] }>(
				`/v1/ai/requests?limit=5&deployment_id=${targetDeployID}`
			);
			aiHistory = res.logs ?? [];
			if (selectedAIHistoryID !== null) {
				const selected = aiHistory.find((entry) => entry.id === selectedAIHistoryID);
				if (selected) {
					selectAIHistoryEntry(selected);
				}
			}
		} catch {
			aiHistory = [];
		}
	}

	function selectAIHistoryEntry(entry: AIRequestLog) {
		selectedAIHistoryID = entry.id;
		aiResult = {
			deployment_id: entry.deployment_id,
			insight: insightTextFromHistory(entry),
			source: entry.provider,
			model: entry.model,
			prompt_version: entry.prompt_version,
			fallback_used: entry.fallback_used,
			confidence: 'medium',
			request_log: entry
		};
	}

	async function generateAIInsight() {
		if (!deployID) return;
		aiBusy = true;
		aiError = '';
		try {
			aiResult = await apiPOST<AIDeployInsightResponse>('/v1/ai/deploy-insights', {
				deployment_id: deployID,
				prompt: aiPrompt
			});
			selectedAIHistoryID = aiResult.request_log?.id ?? null;
			await loadAIHistory(deployID);
		} catch (err) {
			aiError = err instanceof Error ? err.message : 'failed to generate AI insight';
		} finally {
			aiBusy = false;
		}
	}
</script>

<h2>{title}</h2>
<div class="ai-controls">
	<textarea
		bind:value={aiPrompt}
		rows="3"
		placeholder="Ex: Ask AI for a focused deployment analysis"
	></textarea>
	<div class="ai-actions">
		<button on:click={generateAIInsight} disabled={aiBusy || disabled || !deployID}>
			{aiBusy ? 'Generating...' : generateLabel}
		</button>
	</div>
</div>

{#if aiError}
	<p class="error">{aiError}</p>
{/if}

{#if aiResult}
	<article class="ai-result">
		<h3>Latest Insight</h3>
		<div class="ai-markdown" aria-live="polite">
			{@html aiResultHTML}
		</div>
	</article>
{/if}

{#if aiHistory.length > 0}
	<details class="ai-history">
		<summary>{historyLabel}</summary>
		<ul class="logs">
			{#each aiHistory as entry}
				<li>
					<button
						type="button"
						class={`history-item ${selectedAIHistoryID === entry.id ? 'active' : ''}`}
						on:click={() => selectAIHistoryEntry(entry)}
					>
						<span class="stamp">[{prettyDate(entry.created_at)}]</span>
						<span class="level">{entry.status.toUpperCase()}</span>
						<span>{historyInsightPreview(entry)}</span>
					</button>
				</li>
			{/each}
		</ul>
	</details>
{/if}

<style>
	.ai-controls {
		display: grid;
		gap: 0.66rem;
		margin-bottom: 0.92rem;
	}

	.ai-actions {
		display: flex;
		gap: 0.66rem;
		flex-wrap: wrap;
	}

	.ai-result {
		margin-top: 0.56rem;
		margin-bottom: 0.92rem;
	}

	.ai-markdown {
		display: grid;
		gap: 0.5rem;
	}

	.ai-markdown :global(p) {
		margin: 0;
		line-height: 1.5;
	}

	.ai-markdown :global(ul) {
		margin: 0;
		padding-left: 1.1rem;
		display: grid;
		gap: 0.3rem;
	}

	.ai-markdown :global(li) {
		line-height: 1.45;
	}

	.ai-markdown :global(strong) {
		color: var(--text);
		font-weight: 700;
	}

	.ai-markdown :global(em) {
		font-style: italic;
	}

	.ai-markdown :global(code) {
		font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
		font-size: 0.85em;
		background: rgba(138, 173, 244, 0.14);
		border: 1px solid rgba(138, 173, 244, 0.24);
		border-radius: 0.35rem;
		padding: 0.05rem 0.3rem;
	}

	.ai-history {
		margin-top: 0.48rem;
	}

	.history-item {
		width: 100%;
		text-align: left;
		display: grid;
		gap: 0.2rem;
		padding: 0.55rem 0.6rem;
		border: 1px solid rgba(138, 173, 244, 0.24);
		border-radius: 0.6rem;
		background: rgba(36, 39, 58, 0.42);
		color: inherit;
		cursor: pointer;
	}

	.history-item.active {
		border-color: rgba(139, 213, 202, 0.48);
		background: rgba(40, 44, 67, 0.72);
	}
</style>
