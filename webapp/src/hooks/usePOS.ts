import { useEffect, useRef, useState, useCallback } from "react";

export type TransactionStatus = "IDLE" | "PROCESSING" | "SUCCESS" | "FAIL";

// Server state from the state machine
export interface ServerState {
  state: string;
  message: string;
  started_at?: string;
  elapsed_ms: number;
  timeout_ms?: number;
  last_error?: string;
  trans_type?: string;
  amount?: string;
  is_connected: boolean;
}

export interface POSResponse {
  status: "processing" | "success" | "error" | "status_update";
  message: string;
  data?: {
    TransType?: string;
    Amount?: string;
    ApprovalNo?: string;
    MerchantID?: string;
    OrderNo?: string;
    CardNo?: string;
    RespCode?: string;
    // Server state fields
    state?: string;
    is_connected?: boolean;
    elapsed_ms?: number;
    timeout_ms?: number;
    [key: string]: string | number | boolean | undefined;
  };
}

export const usePOS = () => {
  const [status, setStatus] = useState<TransactionStatus>("IDLE");
  const [message, setMessage] = useState<string>("");
  const [lastResult, setLastResult] = useState<POSResponse["data"] | null>(
    null
  );
  const [connected, setConnected] = useState(false);
  const [serverState, setServerState] = useState<ServerState | null>(null);

  const ws = useRef<WebSocket | null>(null);

  useEffect(() => {
    // Connect to Go Server
    const socket = new WebSocket("ws://localhost:8989/ws");

    socket.onopen = () => {
      console.log("Connected to POS Server");
      setConnected(true);
    };

    socket.onclose = () => {
      console.log("Disconnected from POS Server");
      setConnected(false);
      setServerState(null);
    };

    socket.onmessage = (event) => {
      try {
        const resp: POSResponse = JSON.parse(event.data);
        console.log("Received:", resp);

        setMessage(resp.message);

        switch (resp.status) {
          case "status_update":
            // Update server state from broadcast
            if (resp.data) {
              const newState: ServerState = {
                state: resp.data.state || "IDLE",
                message: resp.message,
                elapsed_ms: resp.data.elapsed_ms || 0,
                timeout_ms: resp.data.timeout_ms,
                is_connected: resp.data.is_connected ?? true,
              };
              setServerState(newState);

              // Update connection status
              if (resp.data.is_connected !== undefined) {
                setConnected(resp.data.is_connected);
              }

              // Update UI status based on server state
              if (newState.state === "IDLE") {
                setStatus("IDLE");
              } else if (newState.state === "SUCCESS") {
                setStatus("SUCCESS");
              } else if (
                newState.state === "ERROR" ||
                newState.state === "TIMEOUT"
              ) {
                setStatus("FAIL");
              } else {
                setStatus("PROCESSING");
              }
            }
            break;
          case "processing":
            setStatus("PROCESSING");
            break;
          case "success":
            setStatus("SUCCESS");
            setLastResult(resp.data || null);
            break;
          case "error":
            setStatus("FAIL");
            if (resp.data) {
              setLastResult(resp.data);
            }
            break;
        }
      } catch (e) {
        console.error("Parse error", e);
      }
    };

    ws.current = socket;

    return () => {
      socket.close();
    };
  }, []);

  const sendCommand = useCallback(
    (command: string, amount: string, orderNo?: string) => {
      if (!ws.current || ws.current.readyState !== WebSocket.OPEN) {
        alert("Not connected to POS Server");
        return;
      }

      setStatus("PROCESSING");
      setMessage("Initializing...");
      setLastResult(null);

      ws.current.send(
        JSON.stringify({
          command,
          amount,
          order_no: orderNo,
        })
      );
    },
    []
  );

  const sendAbort = useCallback(() => {
    if (!ws.current || ws.current.readyState !== WebSocket.OPEN) {
      return;
    }
    ws.current.send(JSON.stringify({ command: "ABORT" }));
  }, []);

  const sendReconnect = useCallback(() => {
    if (!ws.current || ws.current.readyState !== WebSocket.OPEN) {
      return;
    }
    ws.current.send(JSON.stringify({ command: "RECONNECT" }));
  }, []);

  const requestStatus = useCallback(() => {
    if (!ws.current || ws.current.readyState !== WebSocket.OPEN) {
      return;
    }
    ws.current.send(JSON.stringify({ command: "STATUS" }));
  }, []);

  const reset = useCallback(() => {
    setStatus("IDLE");
    setMessage("");
    setLastResult(null);
  }, []);

  return {
    connected,
    status,
    message,
    lastResult,
    serverState,
    sendCommand,
    sendAbort,
    sendReconnect,
    requestStatus,
    reset,
  };
};
