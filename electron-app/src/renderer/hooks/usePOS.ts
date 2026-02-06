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
    MerchantOrderNo?: string;
    Invoice_No?: string;
    Reference_No?: string;
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
const DIRECT_WS_URL =
  import.meta.env.VITE_TCB_WS_URL ?? 'ws://127.0.0.1:8989/ws';

// ============ Hook ============

export function usePOS(callbacks: POSCallbacks) {
  const [logs, setLogs] = useState<string[]>([]);
  const callbacksRef = useRef(callbacks);
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const connectRef = useRef<() => void>(() => {});

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
          const merchantOrderNo = resp.data?.MerchantOrderNo;
          const orderNo = merchantOrderNo || resp.data?.OrderNo;
          callbacksRef.current.onTransactionSuccess({
            TransType: resp.data?.TransType,
            Amount: resp.data?.Amount,
            ApprovalNo: resp.data?.ApprovalNo,
            OrderNo: orderNo,
            MerchantOrderNo: merchantOrderNo,
            InvoiceNo: resp.data?.Invoice_No,
            ReferenceNo: resp.data?.Reference_No,
            CardNo: resp.data?.CardNo,
            RespCode: resp.data?.RespCode,
          });
        }
        break;

      case 'error':
        if (resp.command_type === 'transaction') {
          const merchantOrderNo = resp.data?.MerchantOrderNo;
          const orderNo = merchantOrderNo || resp.data?.OrderNo;
          const result: TransactionResult = resp.data
            ? {
                TransType: resp.data?.TransType,
                Amount: resp.data?.Amount,
                ApprovalNo: resp.data?.ApprovalNo,
                OrderNo: orderNo,
                MerchantOrderNo: merchantOrderNo,
                InvoiceNo: resp.data?.Invoice_No,
                ReferenceNo: resp.data?.Reference_No,
                CardNo: resp.data?.CardNo,
                RespCode: resp.data?.RespCode,
              }
            : {};
          callbacksRef.current.onTransactionError(resp.message, result);
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

      addLog(`Connecting to ${DIRECT_WS_URL}`);
      const socket = new WebSocket(DIRECT_WS_URL);
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
        addLog('Auto-reconnecting in 3s...');
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

    connectRef.current = connect;
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
      const generated =
        orderNo && orderNo.trim().length > 0 ? orderNo.trim() : uuid();
      const message = { command, amount, order_no: generated };
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
    addLog('Requesting Transaction Abort...');
    if (isElectron()) {
      await window.electronAPI.ws.send({ command: 'ABORT' });
    } else if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ command: 'ABORT' }));
    }
  }, [addLog]);

  // Send reconnect command
  const sendReconnect = useCallback(async () => {
    if (isElectron()) {
      const status = await window.electronAPI.ws.status();
      if (status.success && status.data?.connected) {
        addLog('Requesting Device Reconnect...');
        await window.electronAPI.ws.send({ command: 'RECONNECT' });
      } else {
        addLog('Attempting Server Reconnect...');
        await window.electronAPI.ws.connect();
      }
      return;
    }

    if (wsRef.current?.readyState === WebSocket.OPEN) {
      addLog('Requesting Device Reconnect...');
      wsRef.current.send(JSON.stringify({ command: 'RECONNECT' }));
    } else {
      addLog('Attempting Server Reconnect...');
      connectRef.current();
    }
  }, [addLog]);

  // Restart Go Server
  const sendRestart = useCallback(async () => {
    if (isElectron()) {
      addLog('Requesting Server Restart...');
      const result = await window.electronAPI.goServer.restart();
      if (!result.success) {
        addLog(`Restart failed: ${result.error}`);
      }
      return;
    }

    if (wsRef.current?.readyState === WebSocket.OPEN) {
      addLog('Requesting Server Restart...');
      wsRef.current.send(JSON.stringify({ command: 'RESTART' }));
    } else {
      addLog('Attempting Server Reconnect...');
      connectRef.current();
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

function uuid(): string {
  // Use Web Crypto if available; fallback to RFC4122 v4 format with Math.random
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) {
    return crypto.randomUUID();
  }
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0;
    const v = c === 'x' ? r : (r & 0x3) | 0x8;
    return v.toString(16);
  });
}
