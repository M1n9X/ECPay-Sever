import { clsx } from "clsx";
import {
  Activity,
  RefreshCw,
  XCircle,
  Server,
  Clock,
  AlertCircle,
  Power,
} from "lucide-react";

interface ServerStateInfo {
  state: string;
  message: string;
  elapsed_ms: number;
  timeout_ms?: number;
  is_connected: boolean;
}

interface ServerStatusProps {
  connected: boolean;
  deviceConnected: boolean;
  serverState: ServerStateInfo | null;
  logs: string[];
  onAbort: () => void;
  onReconnect: () => void;
  onRestart: () => void;
}

const stateColors: Record<string, string> = {
  IDLE: "text-green-400 bg-green-500/10 border-green-500/30",
  SENDING: "text-blue-400 bg-blue-500/10 border-blue-500/30",
  WAIT_ACK: "text-yellow-400 bg-yellow-500/10 border-yellow-500/30",
  WAIT_RESPONSE: "text-orange-400 bg-orange-500/10 border-orange-500/30",
  PARSING: "text-purple-400 bg-purple-500/10 border-purple-500/30",
  SUCCESS: "text-green-400 bg-green-500/10 border-green-500/30",
  ERROR: "text-red-400 bg-red-500/10 border-red-500/30",
  TIMEOUT: "text-red-400 bg-red-500/10 border-red-500/30",
};

export const ServerStatus = ({
  connected,
  deviceConnected,
  serverState,
  logs,
  onAbort,
  onReconnect,
  onRestart,
}: ServerStatusProps) => {
  const currentState = serverState?.state || "IDLE";
  const isProcessing =
    currentState !== "IDLE" &&
    currentState !== "SUCCESS" &&
    currentState !== "ERROR" &&
    currentState !== "TIMEOUT";

  return (
    <div className="bg-zinc-950 border border-zinc-800 rounded-xl p-4 shadow-xl">
      {/* Header */}
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-2">
          <Server className="w-4 h-4 text-zinc-500" />
          <h2 className="text-sm font-bold text-zinc-400 uppercase tracking-widest">
            Server Status
          </h2>
        </div>
      </div>

      {/* Current State */}
      <div className="mb-3">
        <div className="text-xs text-zinc-500 mb-1">Current State</div>
        <div
          className={clsx(
            "inline-flex items-center gap-2 px-3 py-1.5 rounded-lg border text-sm font-mono font-medium",
            stateColors[currentState] || stateColors.IDLE
          )}
        >
          {isProcessing && <Activity className="w-3 h-3 animate-pulse" />}
          {currentState}
        </div>
      </div>

      {/* Progress (if processing) */}
      {isProcessing &&
        serverState?.timeout_ms &&
        serverState.elapsed_ms !== undefined && (
          <div className="mb-3">
            <div className="flex items-center justify-between text-xs text-zinc-500 mb-1">
              <span className="flex items-center gap-1">
                <Clock className="w-3 h-3" />
                Elapsed
              </span>
              <span>
                {Math.floor(serverState.elapsed_ms / 1000)}s /{" "}
                {Math.floor(serverState.timeout_ms / 1000)}s
              </span>
            </div>
            <div className="w-full bg-zinc-800 rounded-full h-1.5">
              <div
                className="bg-blue-500 h-1.5 rounded-full transition-all"
                style={{
                  width: `${Math.min(
                    100,
                    (serverState.elapsed_ms / serverState.timeout_ms) * 100
                  )}%`,
                }}
              />
            </div>
          </div>
        )}

      {/* Message */}
      {serverState?.message && (
        <div className="mb-3 text-xs text-zinc-400 bg-zinc-900 rounded-lg px-3 py-2 border border-zinc-800">
          {serverState.message}
        </div>
      )}

      {/* Control Buttons */}
      <div className="flex gap-2 mb-3">
        <button
          onClick={onReconnect}
          className="flex-1 flex items-center justify-center gap-1.5 bg-blue-500/10 hover:bg-blue-500/20 text-blue-400 px-3 py-2 rounded-lg text-xs font-medium transition-colors border border-blue-500/20"
        >
          <RefreshCw className="w-3 h-3" />
          Server Reconnect
        </button>
        {!deviceConnected ? (
          <button
            onClick={onReconnect}
            disabled={!connected}
            className="flex-1 flex items-center justify-center gap-1.5 bg-yellow-500/10 hover:bg-yellow-500/20 text-yellow-400 px-3 py-2 rounded-lg text-xs font-medium transition-colors border border-yellow-500/20"
          >
            <RefreshCw className="w-3 h-3" />
            Device Reconnect
          </button>
        ) : (
          <button
            onClick={onAbort}
            disabled={!isProcessing}
            className={clsx(
              "flex-1 flex items-center justify-center gap-1.5 px-3 py-2 rounded-lg text-xs font-medium transition-colors border",
              isProcessing
                ? "bg-red-500/10 hover:bg-red-500/20 text-red-400 border-red-500/20"
                : "bg-zinc-800/50 text-zinc-600 border-zinc-700 cursor-not-allowed"
            )}
          >
            <XCircle className="w-3 h-3" />
            Abort
          </button>
        )}
      </div>

      {/* Restart Server Button */}
      <button
        onClick={onRestart}
        className="w-full flex items-center justify-center gap-2 bg-red-600 hover:bg-red-700 text-white px-4 py-2.5 rounded-lg text-sm font-bold transition-colors mb-3 shadow-lg shadow-red-500/20"
      >
        <Power className="w-4 h-4" />
        Restart Server
      </button>

      {/* Logs */}
      <div>
        <div className="text-xs text-zinc-500 mb-1 flex items-center gap-1">
          <AlertCircle className="w-3 h-3" />
          Recent Logs ({logs.length})
        </div>
        <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-2 h-24 overflow-y-auto font-mono text-xs">
          {logs.length === 0 ? (
            <span className="text-zinc-600">No logs yet...</span>
          ) : (
            logs.slice(-10).map((log, i) => (
              <div key={i} className="text-zinc-400 py-0.5">
                {log}
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  );
};
