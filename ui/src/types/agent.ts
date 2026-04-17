export type ApprovedQueryScope = 'none' | 'all' | 'selected';

export interface AgentRecord {
  id: string;
  name: string;
  maskedApiKey: string;
  apiKey?: string | undefined;
  canQuery: boolean;
  canExecute: boolean;
  approvedQueryScope: ApprovedQueryScope;
  approvedQueryIds: string[];
  createdAt?: string | undefined;
  updatedAt?: string | undefined;
  lastUsedAt?: string | undefined;
}

export interface AgentFormValues {
  name: string;
  canQuery: boolean;
  canExecute: boolean;
  approvedQueryScope: ApprovedQueryScope;
  approvedQueryIds: string[];
}
