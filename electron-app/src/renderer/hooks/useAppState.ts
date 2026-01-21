/**
 * Webapp State Machine - Aligned with Server state.go
 *
 * Server States (from state.go):
 *   IDLE -> SENDING -> WAIT_ACK -> WAIT_RESPONSE -> PARSING -> SUCCESS/ERROR/TIMEOUT -> IDLE
 *
 * Webapp combines some server states for simpler UI:
 *   DISCONNECTED: WebSocket not connected
 *   IDLE: Ready for new transaction (server: IDLE)
 *   PROCESSING: Transaction in progress (server: SENDING, WAIT_ACK, WAIT_RESPONSE, PARSING)
 *   SUCCESS: Transaction approved (server: SUCCESS)
 *   ERROR: Transaction failed (server: ERROR)
 *   TIMEOUT: Transaction timed out (server: TIMEOUT)
 */

import { useReducer, useCallback } from "react";

// States aligned with server - but simplified for UI
export type AppState =
  | "DISCONNECTED" // WebSocket not connected
  | "IDLE" // Ready for transaction (server: IDLE)
  | "PROCESSING" // Transaction in progress (server: SENDING/WAIT_ACK/WAIT_RESPONSE/PARSING)
  | "SUCCESS" // Transaction approved (server: SUCCESS)
  | "ERROR" // Transaction failed (server: ERROR)
  | "TIMEOUT"; // Transaction timed out (server: TIMEOUT)

// Server state strings (exactly as defined in state.go)
export type ServerStateString =
  | "IDLE"
  | "SENDING"
  | "WAIT_ACK"
  | "WAIT_RESPONSE"
  | "PARSING"
  | "SUCCESS"
  | "ERROR"
  | "TIMEOUT";

// Map server state to app state
export function serverStateToAppState(
  serverState: ServerStateString
): AppState {
  switch (serverState) {
    case "IDLE":
      return "IDLE";
    case "SENDING":
    case "WAIT_ACK":
    case "WAIT_RESPONSE":
    case "PARSING":
      return "PROCESSING";
    case "SUCCESS":
      return "SUCCESS";
    case "ERROR":
      return "ERROR";
    case "TIMEOUT":
      return "TIMEOUT";
    default:
      return "IDLE";
  }
}

// Form state
export interface FormState {
  tab: "SALE" | "REFUND";
  amount: string;
  orderNo: string;
  refundingOrderId: string | null;
}

// Transaction result
export interface TransactionResult {
  TransType?: string;
  Amount?: string;
  ApprovalNo?: string;
  OrderNo?: string;
  CardNo?: string;
  RespCode?: string;
}

// Complete app state
export interface AppStateData {
  // Connection
  connected: boolean;
  deviceConnected: boolean;

  // Transaction state (aligned with server)
  appState: AppState;
  serverState: ServerStateString; // Raw server state for detailed display
  message: string;
  lastError: string | null;

  // Progress tracking (from server broadcasts)
  elapsed_ms: number;
  timeout_ms: number | null;

  // Transaction result
  lastResult: TransactionResult | null;

  // Form state
  form: FormState;
}

// Events that can trigger state transitions
export type AppEvent =
  | { type: "CONNECT" }
  | { type: "DISCONNECT" }
  | {
      type: "SERVER_STATE_UPDATE";
      state: ServerStateString;
      message: string;
      elapsed_ms: number;
      timeout_ms?: number;
      is_connected: boolean;
    }
  | { type: "TRANSACTION_START" }
  | { type: "TRANSACTION_SUCCESS"; result: TransactionResult }
  | { type: "TRANSACTION_ERROR"; error: string; result?: TransactionResult }
  | { type: "TRANSACTION_TIMEOUT" }
  | { type: "DISMISS" } // Dismiss success/error/timeout modal
  | { type: "RESET_FORM" }
  | { type: "SET_TAB"; tab: "SALE" | "REFUND" }
  | { type: "SET_AMOUNT"; amount: string }
  | { type: "SET_ORDER_NO"; orderNo: string }
  | {
      type: "SET_REFUNDING_ORDER";
      orderId: string;
      amount: string;
      orderNo: string;
    };

// Initial state
const initialState: AppStateData = {
  connected: false,
  deviceConnected: false,
  appState: "DISCONNECTED",
  serverState: "IDLE",
  message: "",
  lastError: null,
  elapsed_ms: 0,
  timeout_ms: null,
  lastResult: null,
  form: {
    tab: "SALE",
    amount: "",
    orderNo: "",
    refundingOrderId: null,
  },
};

// State machine reducer
function reducer(state: AppStateData, event: AppEvent): AppStateData {
  switch (event.type) {
    case "CONNECT":
      return {
        ...state,
        connected: true,
        appState: "IDLE",
        message: "Connected to POS Server",
      };

    case "DISCONNECT":
      return {
        ...state,
        connected: false,
        appState: "DISCONNECTED",
        serverState: "IDLE",
        message: "Disconnected from POS Server",
      };

    case "SERVER_STATE_UPDATE": {
      const newAppState = serverStateToAppState(event.state);
      return {
        ...state,
        serverState: event.state,
        appState: newAppState,
        message: event.message,
        elapsed_ms: event.elapsed_ms,
        timeout_ms: event.timeout_ms ?? null,
        deviceConnected: event.is_connected,
      };
    }

    case "TRANSACTION_START":
      return {
        ...state,
        appState: "PROCESSING",
        message: "Starting transaction...",
        lastResult: null,
        lastError: null,
      };

    case "TRANSACTION_SUCCESS":
      return {
        ...state,
        appState: "SUCCESS",
        message: "Transaction approved",
        lastResult: event.result,
        lastError: null,
      };

    case "TRANSACTION_ERROR":
      return {
        ...state,
        appState: "ERROR",
        message: event.error,
        lastError: event.error,
        lastResult: event.result ?? null,
      };

    case "TRANSACTION_TIMEOUT":
      return {
        ...state,
        appState: "TIMEOUT",
        message: "Transaction timed out",
        lastError: "operation timed out",
      };

    case "DISMISS":
      return {
        ...state,
        appState: state.connected ? "IDLE" : "DISCONNECTED",
        message: "",
        form: {
          ...state.form,
          amount: "",
          orderNo: state.form.refundingOrderId ? "" : state.form.orderNo,
          refundingOrderId: null,
        },
      };

    case "RESET_FORM":
      return {
        ...state,
        form: {
          tab: "SALE",
          amount: "",
          orderNo: "",
          refundingOrderId: null,
        },
      };

    case "SET_TAB":
      return {
        ...state,
        form: {
          ...state.form,
          tab: event.tab,
          orderNo: event.tab === "SALE" ? "" : state.form.orderNo,
          refundingOrderId:
            event.tab === "SALE" ? null : state.form.refundingOrderId,
        },
      };

    case "SET_AMOUNT":
      return {
        ...state,
        form: { ...state.form, amount: event.amount },
      };

    case "SET_ORDER_NO":
      return {
        ...state,
        form: { ...state.form, orderNo: event.orderNo },
      };

    case "SET_REFUNDING_ORDER":
      return {
        ...state,
        form: {
          tab: "REFUND",
          amount: event.amount,
          orderNo: event.orderNo,
          refundingOrderId: event.orderId,
        },
      };

    default:
      return state;
  }
}

// Derived state helpers
export function canSubmit(state: AppStateData): boolean {
  if (!state.connected) return false;
  if (!state.deviceConnected) return false;
  if (state.appState !== "IDLE") return false;

  const amount = parseInt(state.form.amount || "0");
  if (amount <= 0) return false;

  if (state.form.tab === "REFUND" && !state.form.orderNo) return false;

  return true;
}

export function canAbort(state: AppStateData): boolean {
  return state.appState === "PROCESSING";
}

export function canInputForm(state: AppStateData): boolean {
  return state.appState === "IDLE" && state.connected;
}

export function showModal(state: AppStateData): boolean {
  return ["PROCESSING", "SUCCESS", "ERROR", "TIMEOUT"].includes(state.appState);
}

// Hook
export function useAppState() {
  const [state, dispatch] = useReducer(reducer, initialState);

  // Action helpers
  const connect = useCallback(() => dispatch({ type: "CONNECT" }), []);
  const disconnect = useCallback(() => dispatch({ type: "DISCONNECT" }), []);

  const updateServerState = useCallback(
    (
      serverState: ServerStateString,
      message: string,
      elapsed_ms: number,
      timeout_ms?: number,
      is_connected: boolean = false
    ) => {
      dispatch({
        type: "SERVER_STATE_UPDATE",
        state: serverState,
        message,
        elapsed_ms,
        timeout_ms,
        is_connected,
      });
    },
    []
  );

  const startTransaction = useCallback(
    () => dispatch({ type: "TRANSACTION_START" }),
    []
  );

  const transactionSuccess = useCallback((result: TransactionResult) => {
    dispatch({ type: "TRANSACTION_SUCCESS", result });
  }, []);

  const transactionError = useCallback(
    (error: string, result?: TransactionResult) => {
      dispatch({ type: "TRANSACTION_ERROR", error, result });
    },
    []
  );

  const transactionTimeout = useCallback(
    () => dispatch({ type: "TRANSACTION_TIMEOUT" }),
    []
  );

  const dismiss = useCallback(() => dispatch({ type: "DISMISS" }), []);
  const resetForm = useCallback(() => dispatch({ type: "RESET_FORM" }), []);

  const setTab = useCallback((tab: "SALE" | "REFUND") => {
    dispatch({ type: "SET_TAB", tab });
  }, []);

  const setAmount = useCallback((amount: string) => {
    dispatch({ type: "SET_AMOUNT", amount });
  }, []);

  const setOrderNo = useCallback((orderNo: string) => {
    dispatch({ type: "SET_ORDER_NO", orderNo });
  }, []);

  const setRefundingOrder = useCallback(
    (orderId: string, amount: string, orderNo: string) => {
      dispatch({ type: "SET_REFUNDING_ORDER", orderId, amount, orderNo });
    },
    []
  );

  return {
    state,

    // Derived state
    canSubmit: canSubmit(state),
    canAbort: canAbort(state),
    canInputForm: canInputForm(state),
    showModal: showModal(state),

    // Actions
    connect,
    disconnect,
    updateServerState,
    startTransaction,
    transactionSuccess,
    transactionError,
    transactionTimeout,
    dismiss,
    resetForm,
    setTab,
    setAmount,
    setOrderNo,
    setRefundingOrder,
  };
}
