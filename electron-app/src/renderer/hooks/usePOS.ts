/**
 * usePOS - POS Communication Hook
 *
 * Handles communication with the Go Server.
 * - In Electron: Uses IPC bridge via electronAPI
 * - In Browser: Falls back to direct WebSocket (for development)
 */

import { useEffect, useCallback, useState, useRef } from 'react';
import type { ServerStateString, TransactionResult } from './useAppState';

// ============ Types ============

export interface POSResponse {
  status: 'processing' | 'success' | 'error' | 'status_update';
  message: string;
  command_type?: 'transaction' | 'control' | 'status';
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

// ============ Helpers ============

const isElectron = (): boolean => {
  return typeof window !== 'undefined' && 
         window.electronAPI !== undefined;
};

const MAX_LOGS = 50;

// ============ Hook ============

export function usePOS(callbacks: POSCallbacks) {
  const [logs, setLogs] = useState<string[]>([]);
  const callbacksRef = useRef(callbacks);
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Keep callbacks ref updated
  useEffect(() => {
    callbacksRef.current = callbacks;
  }, [callbacks]);

  // Add log entry
  const addLog = useCallback((msg: string) => {
    const timestamp = new Date().toLocaleTimeString();
    setLogs(prev => [...prev.slice(-(MAX_LOGS - 1)), `[${timestamp}] ${msg}`]);
  }, []);

  // Handle incoming message
  const handleMessage = useCallback((data: unknown) => {
    const resp = data as POSResponse;
    addLog(`[${resp.status}] ${resp.message}`);

    switch (resp.status) {
      case 'status_update':
        if (resp.data) {
          callbacksRef.current.onServerStateUpdate(
            (resp.data.state as ServerStateString) || 'IDLE',
            resp.message,
            resp.data.elapsed_ms || 0,
            resp.data.timeout_ms,
            resp.data.is_connected || false
          );
        }
        break;

      case 'success':
        if (resp.command_type === 'transaction') {
          callbacksRef.current.onTransactionSuccess({
            TransType: resp.data?.TransType,
            Amount: resp.data?.Amount,
            ApprovalNo: resp.data?.ApprovalNo,
            OrderNo: resp.data?.OrderNo,
            CardNo: resp.data?.CardNo,
            RespCode: resp.data?.RespCode,
          });
        }
        break;

      case 'error':
        if (resp.command_type === 'transaction') {
          callbacksRef.current.onTransactionError(resp.message, resp.data ? {
            TransType: resp.data?.TransType,
            Amount: resp.data?.Amount,
            ApprovalNo: resp.data?.ApprovalNo,
            OrderNo: resp.data?.OrderNo,
            CardNo: resp.data?.CardNo,
            RespCode: resp.data?.RespCode,
          } : undefined);
        }
        break;
    }
  }, [addLog]);

  // Setup Electron IPC listeners
  useEffect(() => {
    if (!isElectron()) {
      // Fallback to direct WebSocket for browser development
      addLog('Browser mode: using direct WebSocket');
      return setupDirectWebSocket();
    }

    addLog('Electron mode: using IPC bridge');
    const api = window.electronAPI;

    // Setup event listeners
    const cleanups = [
      api.ws.onConnected(() => {
        addLog('Connected to POS Server');
        callbacksRef.current.onConnect();
      }),
      api.ws.onDisconnected(() => {
        addLog('Disconnected from POS Server');
        callbacksRef.current.onDisconnect();
      }),
      api.ws.onMessage(handleMessage),
      api.ws.onError((error) => {
        addLog(`WebSocket error: ${error}`);
      }),
      api.goServer.onLog((log) => {
        addLog(`[Go ${log.level}] ${log.message}`);
      }),
    ];

    // Initial connection
    api.ws.connect();

    return () => {
      cleanups.forEach(cleanup => cleanup());
    };
  }, [addLog, handleMessage]);

  // Direct WebSocket fallback for browser development
  const setupDirectWebSocket = useCallback(() => {
    const connect = () => {
      if (wsRef.current?.readyState === WebSocket.OPEN) return;

      addLog('Connecting to ws://localhost:8989/ws');
      const socket = new WebSocket('ws://localhost:8989/ws');
      wsRef.current = socket;

      socket.onopen = () => {
        addLog('Connected to POS Server');
        callbacksRef.current.onConnect();
      };

      socket.onclose = () => {
        addLog('Disconnected from POS Server');
        callbacksRef.current.onDisconnect();
        wsRef.current = null;

        // Auto-reconnect
        reconnectTimerRef.current = setTimeout(connect, 3000);
      };

      socket.onerror = () => {
        addLog('WebSocket error');
      };

      socket.onmessage = (event) => {
        try {
          handleMessage(JSON.parse(event.data));
        } catch {
          addLog('Failed to parse message');
        }
      };
    };

    connect();

    return () => {
      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current);
      }
      if (wsRef.current) {
        wsRef.current.onclose = null;
        wsRef.current.close();
        wsRef.current = null;
      }
    };
  }, [addLog, handleMessage]);

  // Send transaction command
  const sendTransaction = useCallback(
    async (command: 'SALE' | 'REFUND', amount: string, orderNo?: string) => {
      const message = { command, amount, order_no: orderNo };
      addLog(`Sending ${command}: $${(parseInt(amount) / 100).toFixed(2)}`);

      if (isElectron()) {
        const result = await window.electronAPI.ws.send(message);
        if (!result.success) {
          addLog(`Send failed: ${result.error}`);
          return false;
        }
        return true;
      } else {
        // Direct WebSocket
        if (wsRef.current?.readyState === WebSocket.OPEN) {
          wsRef.current.send(JSON.stringify(message));
          return true;
        }
        addLog('Not connected');
        return false;
      }
    },
    [addLog]
  );

  // Send abort command
  const sendAbort = useCallback(async () => {
    addLog('Requesting abort...');
    if (isElectron()) {
      await window.electronAPI.ws.send({ command: 'ABORT' });
    } else if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ command: 'ABORT' }));
    }
  }, [addLog]);

  // Send reconnect command
  const sendReconnect = useCallback(async () => {
    addLog('Requesting device reconnect...');
    if (isElectron()) {
      await window.electronAPI.ws.send({ command: 'RECONNECT' });
    } else if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ command: 'RECONNECT' }));
    }
  }, [addLog]);

  // Restart Go Server
  const sendRestart = useCallback(async () => {
    addLog('Restarting Go Server...');
    if (isElectron()) {
      const result = await window.electronAPI.goServer.restart();
      if (!result.success) {
        addLog(`Restart failed: ${result.error}`);
      }
    } else {
      addLog('Restart not available in browser mode');
    }
  }, [addLog]);

  // Request status
  const requestStatus = useCallback(async () => {
    if (isElectron()) {
      await window.electronAPI.ws.send({ command: 'STATUS' });
    } else if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ command: 'STATUS' }));
    }
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
