/**
 * usePOS - WebSocket Communication Hook
 *
 * This hook handles only WebSocket communication with the server.
 * State management is delegated to useAppState.
 */

import { useEffect, useRef, useCallback, useState } from "react";
import type { ServerStateString, TransactionResult } from "./useAppState";

export interface POSResponse {
  status: "processing" | "success" | "error" | "status_update";
  message: string;
  command_type?: "transaction" | "control" | "status";
  data?: {
    TransType?: string;
    Amount?: string;
    ApprovalNo?: string;
    MerchantID?: string;
    OrderNo?: string;
    CardNo?: string;
    RespCode?: string;
    state?: string;
    is_connected?: boolean;
    elapsed_ms?: number;
    timeout_ms?: number;
    [key: string]: string | number | boolean | undefined;
  };
}

export interface POSCallbacks {
  onConnect: () => void;
  onDisconnect: () => void;
  onServerStateUpdate: (
    state: ServerStateString,
    message: string,
    elapsed_ms: number,
    timeout_ms?: number,
    is_connected?: boolean
  ) => void;
  onTransactionSuccess: (result: TransactionResult) => void;
  onTransactionError: (error: string, result?: TransactionResult) => void;
}

export function usePOS(callbacks: POSCallbacks) {
  const ws = useRef<WebSocket | null>(null);
  const [logs, setLogs] = useState<string[]>([]);
  const [reconnectCount, setReconnectCount] = useState(0);

  const addLog = useCallback((msg: string) => {
    const timestamp = new Date().toLocaleTimeString();
    setLogs((prev) => [...prev.slice(-49), `[${timestamp}] ${msg}`]);
  }, []);

  // Connect to WebSocket
  useEffect(() => {
    let socket: WebSocket;
    let reconnectTimer: ReturnType<typeof setTimeout>;

    const connect = () => {
      socket = new WebSocket("ws://localhost:8989/ws");

      socket.onopen = () => {
        console.log("Connected to POS Server");
        addLog("Connected to POS Server");
        callbacks.onConnect();
      };

      socket.onclose = () => {
        console.log("Disconnected from POS Server");
        addLog("Disconnected from POS Server");
        callbacks.onDisconnect();

        // Auto-reconnect after 3 seconds
        addLog("Auto-reconnecting in 3s...");
        reconnectTimer = setTimeout(() => {
          setReconnectCount((prev) => prev + 1);
        }, 3000);
      };

      socket.onerror = (error) => {
        console.error("WebSocket error:", error);
        addLog("WebSocket error");
      };

      socket.onmessage = (event) => {
        try {
          const resp: POSResponse = JSON.parse(event.data);
          console.log("Received:", resp);
          // Log everything
          addLog(`[${resp.status}] ${resp.message}`);

          // Handle different response types
          switch (resp.status) {
            case "status_update":
              if (resp.data) {
                callbacks.onServerStateUpdate(
                  (resp.data.state as ServerStateString) || "IDLE",
                  resp.message,
                  resp.data.elapsed_ms || 0,
                  resp.data.timeout_ms,
                  resp.data.is_connected || false
                );
              }
              break;

            case "success":
              // Handle transaction responses
              if (resp.command_type === "transaction") {
                const result: TransactionResult = {
                  TransType: resp.data?.TransType,
                  Amount: resp.data?.Amount,
                  ApprovalNo: resp.data?.ApprovalNo,
                  OrderNo: resp.data?.OrderNo,
                  CardNo: resp.data?.CardNo,
                  RespCode: resp.data?.RespCode,
                };
                callbacks.onTransactionSuccess(result);
              }
              // Handle control responses (e.g. Abort success)
              else if (resp.command_type === "control") {
                // For now just allow the log to stand
              }
              break;

            case "error":
              // Only handle transaction responses
              if (resp.command_type === "transaction") {
                const result: TransactionResult = resp.data
                  ? {
                      TransType: resp.data?.TransType,
                      Amount: resp.data?.Amount,
                      ApprovalNo: resp.data?.ApprovalNo,
                      OrderNo: resp.data?.OrderNo,
                      CardNo: resp.data?.CardNo,
                      RespCode: resp.data?.RespCode,
                    }
                  : {};
                callbacks.onTransactionError(resp.message, result);
              }
              break;

            case "processing":
              // Processing notifications are informational, state update will follow
              break;
          }
        } catch (e) {
          console.error("Parse error:", e);
          addLog("Parse error");
        }
      };

      ws.current = socket;
    };

    connect();

    return () => {
      if (socket) {
        socket.onclose = null; // Prevent triggering auto-reconnect on cleanup
        socket.close();
      }
      clearTimeout(reconnectTimer);
    };
  }, [callbacks, addLog, reconnectCount]);

  // Send transaction command
  const sendTransaction = useCallback(
    (command: "SALE" | "REFUND", amount: string, orderNo?: string) => {
      if (!ws.current || ws.current.readyState !== WebSocket.OPEN) {
        console.error("Not connected to POS Server");
        return false;
      }

      addLog(`Sending ${command}: $${(parseInt(amount) / 100).toFixed(2)}`);
      ws.current.send(
        JSON.stringify({
          command,
          amount,
          order_no: orderNo,
        })
      );
      return true;
    },
    [addLog]
  );

  // Send control commands
  const sendAbort = useCallback(() => {
    if (!ws.current || ws.current.readyState !== WebSocket.OPEN) return;
    addLog("Requesting Transaction Abort...");
    ws.current.send(JSON.stringify({ command: "ABORT" }));
  }, [addLog]);

  const sendReconnect = useCallback(() => {
    // If connected, send command to server to reconnect to POS
    if (ws.current && ws.current.readyState === WebSocket.OPEN) {
      addLog("Requesting Device Reconnect...");
      ws.current.send(JSON.stringify({ command: "RECONNECT" }));
    } else {
      // If disconnected, try to reconnect WebSocket
      addLog("Attempting Server Reconnect...");
      setReconnectCount((c) => c + 1);
    }
  }, [addLog]);

  const sendRestart = useCallback(() => {
    if (ws.current && ws.current.readyState === WebSocket.OPEN) {
      addLog("Requesting Server Restart...");
      ws.current.send(JSON.stringify({ command: "RESTART" }));
    } else {
      // If disconnected, we can't tell server to restart, but we can try to reconnect WS
      // Maybe the server is back up?
      addLog("Attempting Server Reconnect...");
      setReconnectCount((c) => c + 1);
    }
  }, [addLog]);

  const requestStatus = useCallback(() => {
    if (!ws.current || ws.current.readyState !== WebSocket.OPEN) return;
    ws.current.send(JSON.stringify({ command: "STATUS" }));
  }, []);

  return {
    logs,
    sendTransaction,
    sendAbort,
    sendReconnect,
    sendRestart,
    requestStatus,
  };
}
