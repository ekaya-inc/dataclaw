export type MCPToolEventType = 'tool_call' | 'tool_error';
export type MCPToolEventRange = '24h' | '7d' | '30d' | 'all';

export interface MCPToolEventRecord {
  id: string;
  agentId?: string | undefined;
  agentName: string;
  toolName: string;
  eventType: MCPToolEventType;
  wasSuccessful: boolean;
  durationMs: number;
  hasDetails: boolean;
  createdAt: string;
}

export interface MCPToolEventDetails {
  id: string;
  requestParams: Record<string, unknown>;
  resultSummary: Record<string, unknown>;
  errorMessage: string;
  queryName: string;
  sqlText: string;
}

export interface MCPToolEventPage {
  items: MCPToolEventRecord[];
  total: number;
  limit: number;
  offset: number;
}

export interface MCPToolEventFilters {
  range?: MCPToolEventRange | undefined;
  eventType?: MCPToolEventType | '' | undefined;
  toolName?: string | undefined;
  agentName?: string | undefined;
  limit?: number | undefined;
  offset?: number | undefined;
}
