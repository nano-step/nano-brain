import { loadCollectionConfig, listCollections } from './collections.js';
import { WorkspaceProfile } from './workspace-profile.js';
import type { Store } from './types.js';
import { log } from './logger.js';

export interface BriefingSection {
  label: string;
  items: string[];
}

export interface BriefingResult {
  workspace: string;
  l0: BriefingSection;
  l1_memories: BriefingSection;
  l1_decisions: BriefingSection;
  formatted: string;
}

export interface BriefingOptions {
  limit?: number;
  decisionLimit?: number;
  maxChars?: number;
  json?: boolean;
}

export function generateBriefing(
  store: Store,
  configPath: string,
  projectHash: string,
  options: BriefingOptions = {}
): BriefingResult {
  const limit = options.limit ?? 10;
  const decisionLimit = options.decisionLimit ?? 5;
  const maxChars = options.maxChars ?? 2000;

  const config = loadCollectionConfig(configPath);
  const collectionNames = config ? listCollections(config).join(', ') : 'none';

  let topicsLine = '';
  try {
    const profile = new WorkspaceProfile(store);
    const data = profile.loadProfile(projectHash);
    if (data?.topTopics && data.topTopics.length > 0) {
      topicsLine = data.topTopics.slice(0, 5).map(t => t.topic).join(', ');
    }
  } catch (err) {
    log('wake-up', 'Failed to load workspace profile: ' + (err instanceof Error ? err.message : String(err)));
  }

  const topDocs = store.getTopAccessedDocuments(limit, projectHash);
  const decisions = store.getRecentDocumentsByTags(['decision'], decisionLimit, projectHash);

  const l0: BriefingSection = {
    label: 'Workspace Identity',
    items: [
      `Collections: ${collectionNames}` + (topicsLine ? ` | Topics: ${topicsLine}` : ''),
    ],
  };

  const l1_memories: BriefingSection = {
    label: 'Key Memories',
    items: topDocs.map(d => {
      const snippet = truncateLine(d.title || d.path, 80);
      return `${snippet} (${d.collection}) — accessed ${d.access_count}x`;
    }),
  };

  const l1_decisions: BriefingSection = {
    label: 'Recent Decisions',
    items: decisions.map(d => {
      const date = d.modified_at?.split('T')[0] || 'unknown';
      const snippet = truncateLine(d.title || d.path, 80);
      return `${snippet} (${date})`;
    }),
  };

  const workspaceName = configPath.split('/').filter(Boolean).pop() || projectHash;
  let formatted = formatBriefing(workspaceName, l0, l1_memories, l1_decisions);

  if (topDocs.length === 0 && decisions.length === 0) {
    formatted = `## Context Briefing — ${workspaceName}\nNo memories yet. Start by writing notes or indexing a codebase.`;
  } else if (formatted.length > maxChars) {
    formatted = formatted.slice(0, maxChars - 3) + '...';
  }

  return { workspace: workspaceName, l0, l1_memories, l1_decisions, formatted };
}

function formatBriefing(
  name: string,
  l0: BriefingSection,
  memories: BriefingSection,
  decisions: BriefingSection
): string {
  const lines: string[] = [];
  lines.push(`## Context Briefing — ${name}`);
  lines.push(l0.items.join(' | '));
  lines.push('');

  if (memories.items.length > 0) {
    lines.push('### Key Memories');
    for (const item of memories.items) {
      lines.push(`- ${item}`);
    }
    lines.push('');
  }

  if (decisions.items.length > 0) {
    lines.push('### Recent Decisions');
    for (const item of decisions.items) {
      lines.push(`- ${item}`);
    }
  }

  return lines.join('\n');
}

function truncateLine(text: string, max: number): string {
  if (!text) return '(untitled)';
  if (text.length <= max) return text;
  return text.slice(0, max - 3) + '...';
}
