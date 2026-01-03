// Service configuration from csd-core
export interface ServiceConfig {
  graphqlUrl: string;
  coreGraphqlUrl: string;
}

// Project types
export interface Project {
  id: string;
  name: string;
  type: string;
  componentCount: number;
  gitBranch: string;
  gitDirty: boolean;
  runningCount: number;
}

// Session types
export interface SessionSummary {
  id: string;
  name: string;
  projectId: string;
  projectName: string;
  workDir: string;
  claudeProjectDir: string;
  state: 'idle' | 'running' | 'waiting' | 'error';
  messageCount: number;
  createdAt: string;
  lastActiveAt: string;
}

export interface Session {
  id: string;
  name: string;
  projectId: string;
  projectName: string;
  workDir: string;
  state: 'idle' | 'running' | 'waiting' | 'error';
  messages: Message[];
  createdAt: string;
  lastActiveAt: string;
  error?: string;
  isRealSession: boolean;
  sessionFile: string;
}

export interface Message {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  timestamp: string;
  partial: boolean;
}
