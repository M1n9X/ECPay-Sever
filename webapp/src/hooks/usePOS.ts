import { useEffect, useRef, useState, useCallback } from "react";

export type TransactionStatus = "IDLE" | "PROCESSING" | "SUCCESS" | "FAIL";

export interface POSResponse {
  status: "processing" | "success" | "error";
  message: string;
  data?: {
    TransType: string;
    Amount: string;
    ApprovalNo: string;
    MerchantID: string;
    OrderNo: string;
    CardNo: string;
    RespCode: string;
    [key: string]: string;
  };
}

export const usePOS = () => {
  const [status, setStatus] = useState<TransactionStatus>("IDLE");
  const [message, setMessage] = useState<string>("");
  const [lastResult, setLastResult] = useState<POSResponse["data"] | null>(
    null
  );
  const [connected, setConnected] = useState(false);

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
    };

    socket.onmessage = (event) => {
      try {
        const resp: POSResponse = JSON.parse(event.data);
        console.log("Received:", resp);

        setMessage(resp.message);

        switch (resp.status) {
          case "processing":
            setStatus("PROCESSING");
            break;
          case "success":
            setStatus("SUCCESS");
            setLastResult(resp.data || null);
            break;
          case "error":
            setStatus("FAIL");
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
    sendCommand,
    reset,
  };
};
