import type { AIRequestLog } from '$lib/api';

export function insightTextFromHistory(entry: AIRequestLog): string {
	const full = entry.output_text?.trim() ?? '';
	if (full.length > 0) return full;
	const excerpt = entry.output_excerpt?.trim() ?? '';
	if (excerpt.length > 0) return excerpt;
	return 'No AI output was stored for this request.';
}

export function historyInsightPreview(entry: AIRequestLog): string {
	const normalized = insightTextFromHistory(entry)
		.replace(/\r\n?/g, '\n')
		.replace(/^\s*[-*]\s+/gm, '')
		.replace(/\*\*/g, '')
		.replace(/`/g, '')
		.replace(/\s+/g, ' ')
		.trim();
	if (normalized.length <= 180) return normalized;
	return `${normalized.slice(0, 177)}...`;
}

function escapeHTML(value: string): string {
	return value
		.replaceAll('&', '&amp;')
		.replaceAll('<', '&lt;')
		.replaceAll('>', '&gt;')
		.replaceAll('"', '&quot;')
		.replaceAll("'", '&#39;');
}

function renderInlineMarkdown(value: string): string {
	const escaped = escapeHTML(value);
	return escaped
		.replace(/`([^`]+)`/g, '<code>$1</code>')
		.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
		.replace(/\*([^*\n]+)\*/g, '<em>$1</em>');
}

export function renderInsightMarkdown(value: string): string {
	const normalized = value.replace(/\r\n?/g, '\n').trim();
	if (!normalized) return '<p>No AI output was returned.</p>';

	const lines = normalized.split('\n');
	const html: string[] = [];
	let paragraph: string[] = [];
	let listItems: string[] = [];

	const flushParagraph = () => {
		if (paragraph.length === 0) return;
		const text = paragraph.join(' ').trim();
		if (text) html.push(`<p>${renderInlineMarkdown(text)}</p>`);
		paragraph = [];
	};

	const flushList = () => {
		if (listItems.length === 0) return;
		const items = listItems.map((item) => `<li>${renderInlineMarkdown(item)}</li>`).join('');
		html.push(`<ul>${items}</ul>`);
		listItems = [];
	};

	for (const rawLine of lines) {
		const line = rawLine.trim();
		if (!line) {
			flushParagraph();
			flushList();
			continue;
		}

		const bulletMatch = line.match(/^[-*]\s+(.+)$/);
		if (bulletMatch) {
			flushParagraph();
			listItems.push(bulletMatch[1].trim());
			continue;
		}

		if (listItems.length > 0) flushList();
		paragraph.push(line);
	}

	flushParagraph();
	flushList();
	return html.join('');
}
