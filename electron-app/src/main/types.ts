/**
 * Shared Types for Main Process
 */

// IPC Response types
export interface IpcResponse<T = unknown> {
  success: boolean;
  data?: T;
  error?: string;
}

// Go Server types
export interface GoServerStatus {
  running: boolean;
  pid?: number;
  uptime?: number;
}

export interface GoServerLog {
  level: 'INFO' | 'WARN' | 'ERROR';
  message: string;
  timestamp?: number;
}

export interface GoServerExitInfo {
  code: number | null;
  signal: NodeJS.Signals | null;
}

// WebSocket types
export interface WsMessage {
  command?: string;
  amount?: string;
  order_no?: string;
  [key: string]: unknown;
}

// Process Manager events
export interface ProcessManagerEvents {
  ready: () => void;
  exit: (info: GoServerExitInfo) => void;
  error: (error: Error) => void;
  log: (log: GoServerLog) => void;
}
