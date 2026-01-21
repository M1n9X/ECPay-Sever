import { useEffect, useRef, useMemo, useCallback } from "react";
import {
  useAppState,
  type TransactionResult,
  type ServerStateString,
} from "./hooks/useAppState";
import { usePOS, type POSCallbacks } from "./hooks/usePOS";
import { useOrders } from "./hooks/useOrders";
import type { Order } from "./hooks/useOrders";
import { Keypad } from "./components/Keypad";
import { OrderHistory } from "./components/OrderHistory";
import { ServerStatus } from "./components/ServerStatus";
import {
  CreditCard,
  Terminal,
  CheckCircle2,
  RotateCcw,
  AlertTriangle,
  History,
  Wifi,
  WifiOff,
  XCircle,
  Activity,
  Clock,
} from "lucide-react";
import { clsx } from "clsx";

function App() {
  // State machine
  const {
    state,
    canSubmit,
    canAbort,
    canInputForm,
    showModal,
    connect,
    disconnect,
    updateServerState,
    transactionSuccess,
    transactionError,
    dismiss,
    setTab,
    setAmount,
    setOrderNo,
    setRefundingOrder,
  } = useAppState();

  // Order management
  const { orders, addOrder, markRefunded } = useOrders();

  // Track processed transactions to avoid duplicates
  const processedApprovalRef = useRef<string | null>(null);

  // POS callbacks
  const posCallbacks: POSCallbacks = useMemo(
    () => ({
      onConnect: connect,
      onDisconnect: disconnect,
      onServerStateUpdate: (
        serverState: ServerStateString,
        message: string,
        elapsed_ms: number,
        timeout_ms?: number,
        is_connected?: boolean
      ) => {
        updateServerState(serverState, message, elapsed_ms, timeout_ms, is_connected);
      },
      onTransactionSuccess: (result: TransactionResult) => {
        transactionSuccess(result);
      },
      onTransactionError: (error: string, result?: TransactionResult) => {
        transactionError(error, result);
      },
    }),
    [
      connect,
      disconnect,
      updateServerState,
      transactionSuccess,
      transactionError,
    ]
  );

  // WebSocket communication (via IPC in Electron)
  const { logs, sendTransaction, sendAbort, sendReconnect, sendRestart } =
    usePOS(posCallbacks);

  // Handle order saving on success
  useEffect(() => {
    if (state.appState === "SUCCESS" && state.lastResult) {
      const currentApproval = state.lastResult.ApprovalNo ?? null;
      if (processedApprovalRef.current === currentApproval) {
        return;
      }
      processedApprovalRef.current = currentApproval;

      const orderData = {
        type: (state.lastResult.TransType === "01" ? "SALE" : "REFUND") as
          | "SALE"
          | "REFUND",
        amount: parseInt(state.lastResult.Amount || "0"),
        orderNo: state.lastResult.OrderNo || "",
        approvalNo: state.lastResult.ApprovalNo || "",
        cardNo: state.lastResult.CardNo || "",
      };
      addOrder(orderData);

      // Mark original order as refunded if applicable
      if (state.form.refundingOrderId && orderData.type === "REFUND") {
        markRefunded(state.form.refundingOrderId);
      }
    }
  }, [
    state.appState,
    state.lastResult,
    state.form.refundingOrderId,
    addOrder,
    markRefunded,
  ]);

  // Form handlers
  const handleInput = useCallback(
    (val: string) => {
      if (state.form.amount.length > 8) return;
      setAmount(state.form.amount + val);
    },
    [state.form.amount, setAmount]
  );

  const handleClear = useCallback(() => setAmount(""), [setAmount]);

  const handleDelete = useCallback(() => {
    setAmount(state.form.amount.slice(0, -1));
  }, [state.form.amount, setAmount]);

  const handleCheckout = useCallback(() => {
    if (!canSubmit) return;
    sendTransaction(
      state.form.tab,
      state.form.amount,
      state.form.tab === "REFUND" ? state.form.orderNo : undefined
    );
  }, [canSubmit, sendTransaction, state.form]);

  const handleRefundOrder = useCallback(
    (order: Order) => {
      setRefundingOrder(order.id, order.amount.toString(), order.orderNo);
    },
    [setRefundingOrder]
  );

  const handleTabChange = useCallback(
    (tab: "SALE" | "REFUND") => {
      if (!canInputForm) return;
      setTab(tab);
    },
    [canInputForm, setTab]
  );

  const handleDismiss = useCallback(() => {
    dismiss();
    processedApprovalRef.current = null;
  }, [dismiss]);

  // Derived values for display
  const amountDisplay = (parseInt(state.form.amount || "0") / 100).toFixed(2);

  return (
    <div className="h-screen bg-black text-white flex flex-col lg:flex-row items-center lg:items-start justify-center gap-4 p-3 lg:p-4 overflow-hidden">
      {/* Connection Status Header */}
      <div className="fixed top-4 left-4 right-4 flex items-center justify-between z-40">
        <div className="flex items-center gap-3 bg-zinc-900/80 backdrop-blur-md px-4 py-2 rounded-full border border-zinc-800">
          <div className="flex items-center gap-4">
            <div className="flex items-center gap-2">
              {state.connected ? (
                <Wifi className="w-4 h-4 text-green-500" />
              ) : (
                <WifiOff className="w-4 h-4 text-red-500" />
              )}
              <span
                className={clsx(
                  "text-sm font-medium",
                  state.connected ? "text-green-400" : "text-red-400"
                )}
              >
                {state.connected ? "Server Connected" : "Server Disconnected"}
              </span>
            </div>
            {state.connected && (
              <div className="flex items-center gap-2">
                <div
                  className={clsx(
                    "w-2 h-2 rounded-full",
                    state.deviceConnected ? "bg-green-500" : "bg-yellow-500"
                  )}
                />
                <span
                  className={clsx(
                    "text-sm font-medium",
                    state.deviceConnected ? "text-green-400" : "text-yellow-400"
                  )}
                >
                  {state.deviceConnected
                    ? "Device Connected"
                    : "Device Missing"}
                </span>
              </div>
            )}
          </div>
          {/* Server State Indicator - Show when processing */}
          {state.appState === "PROCESSING" && (
            <>
              <span className="text-zinc-600">|</span>
              <Activity className="w-3 h-3 text-blue-400 animate-pulse" />
              <span className="text-xs font-mono text-blue-400">
                {state.serverState}
              </span>
              {state.timeout_ms && state.elapsed_ms > 0 && (
                <span className="text-xs text-zinc-500">
                  ({Math.floor(state.elapsed_ms / 1000)}s /{" "}
                  {Math.floor(state.timeout_ms / 1000)}s)
                </span>
              )}
            </>
          )}
        </div>
        {/* Control Buttons (Header) */}
        <div className="flex items-center gap-2">
          {canAbort && (
            <button
              onClick={sendAbort}
              className="flex items-center gap-1.5 bg-red-500/20 hover:bg-red-500/30 text-red-400 px-3 py-1.5 rounded-full text-xs font-medium transition-colors border border-red-500/30"
            >
              <XCircle className="w-3 h-3" />
              Abort
            </button>
          )}
        </div>
      </div>

      {/* Main Card */}
      <div className="max-w-md w-full mt-12 lg:mt-2">
        <div className="bg-zinc-950 border border-zinc-800 rounded-2xl p-5 lg:p-6 shadow-2xl relative overflow-hidden">
          {/* Background Gradient */}
          <div className="absolute top-0 left-0 right-0 h-32 bg-gradient-to-b from-blue-500/10 to-transparent pointer-events-none" />

          {/* Title */}
          <div className="flex items-center justify-between mb-4 relative z-10">
            <div>
              <h1 className="text-2xl font-bold bg-clip-text text-transparent bg-gradient-to-r from-white to-zinc-500">
                ECPay POS
              </h1>
              <p className="text-zinc-500 text-sm">RS232 Terminal</p>
            </div>
            <Terminal className="text-blue-500 w-7 h-7 opacity-80" />
          </div>

          {/* Tabs */}
          <div className="flex p-1 bg-zinc-900 rounded-xl mb-4 border border-zinc-800">
            <button
              onClick={() => handleTabChange("SALE")}
              disabled={!canInputForm}
              className={clsx(
                "flex-1 flex items-center justify-center gap-2 py-2.5 rounded-lg text-sm font-medium transition-all",
                state.form.tab === "SALE"
                  ? "bg-zinc-800 text-white shadow-lg"
                  : "text-zinc-500 hover:text-zinc-300",
                !canInputForm && "opacity-50 cursor-not-allowed"
              )}
            >
              <CreditCard className="w-4 h-4" />
              Sale
            </button>
            <button
              onClick={() => handleTabChange("REFUND")}
              disabled={!canInputForm}
              className={clsx(
                "flex-1 flex items-center justify-center gap-2 py-2.5 rounded-lg text-sm font-medium transition-all",
                state.form.tab === "REFUND"
                  ? "bg-zinc-800 text-white shadow-lg"
                  : "text-zinc-500 hover:text-zinc-300",
                !canInputForm && "opacity-50 cursor-not-allowed"
              )}
            >
              <RotateCcw className="w-4 h-4" />
              Refund
            </button>
          </div>

          {/* Refund Order Input */}
          {state.form.tab === "REFUND" && (
            <div className="mb-3">
              <label className="text-xs font-bold text-zinc-500 uppercase tracking-widest pl-1">
                Original Order No
              </label>
              <input
                type="text"
                value={state.form.orderNo}
                onChange={(e) => setOrderNo(e.target.value)}
                disabled={!canInputForm}
                placeholder="Enter Order No"
                className={clsx(
                  "w-full mt-1 bg-zinc-900 border border-zinc-800 rounded-xl px-4 py-2.5 text-white focus:outline-none focus:ring-2 focus:ring-blue-500/50 transition-all font-mono text-sm",
                  !canInputForm && "opacity-50 cursor-not-allowed"
                )}
              />
            </div>
          )}

          {/* Keypad */}
          <Keypad
            onInput={handleInput}
            onClear={handleClear}
            onDelete={handleDelete}
            amount={state.form.amount}
            disabled={!canInputForm}
          />

          {/* Action Button */}
          <button
            onClick={handleCheckout}
            disabled={!canSubmit}
            className="w-full mt-4 py-3.5 rounded-xl bg-gradient-to-r from-blue-600 to-indigo-600 font-bold text-lg shadow-lg shadow-blue-500/20 hover:scale-[1.02] active:scale-[0.98] transition-all disabled:opacity-50 disabled:pointer-events-none"
          >
            {state.form.tab === "SALE" ? "Charge" : "Refund"} ${amountDisplay}
          </button>
        </div>
      </div>

      {/* Right Panel */}
      <div className="w-full max-w-md lg:w-72 mt-2 lg:mt-12 flex flex-col gap-4">
        {/* Order History Panel */}
        <div className="bg-zinc-950 border border-zinc-800 rounded-xl p-4 shadow-xl">
          <div className="flex items-center gap-2 mb-3">
            <History className="w-4 h-4 text-zinc-500" />
            <h2 className="text-sm font-bold text-zinc-400 uppercase tracking-widest">
              Recent Transactions
            </h2>
          </div>
          <OrderHistory orders={orders} onRefund={handleRefundOrder} />
        </div>

        {/* Server Status Panel */}
        <ServerStatus
          connected={state.connected}
          serverState={{
            state: state.serverState,
            message: state.message,
            elapsed_ms: state.elapsed_ms,
            timeout_ms: state.timeout_ms ?? undefined,
            is_connected: state.deviceConnected,
          }}
          deviceConnected={state.deviceConnected}
          logs={logs}
          onAbort={sendAbort}
          onReconnect={sendReconnect}
          onRestart={sendRestart}
        />
      </div>

      {/* Status Overlay Modal */}
      {showModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-sm p-4">
          <div className="bg-zinc-900 border border-zinc-800 rounded-3xl p-8 max-w-sm w-full shadow-2xl flex flex-col items-center text-center">
            {state.appState === "PROCESSING" && (
              <>
                <div className="w-16 h-16 rounded-full border-4 border-blue-500/30 border-t-blue-500 animate-spin mb-6" />
                <h3 className="text-xl font-bold mb-2">Processing</h3>
                <p className="text-zinc-400 mb-2">{state.message}</p>
                {/* Progress display */}
                {state.timeout_ms && state.elapsed_ms > 0 && (
                  <div className="w-full mt-4">
                    <div className="flex items-center justify-between text-xs text-zinc-500 mb-1">
                      <span className="flex items-center gap-1">
                        <Clock className="w-3 h-3" />
                        Elapsed
                      </span>
                      <span>
                        {Math.floor(state.elapsed_ms / 1000)}s /{" "}
                        {Math.floor(state.timeout_ms / 1000)}s
                      </span>
                    </div>
                    <div className="w-full bg-zinc-800 rounded-full h-1.5">
                      <div
                        className="bg-blue-500 h-1.5 rounded-full transition-all"
                        style={{
                          width: `${Math.min(
                            100,
                            (state.elapsed_ms / state.timeout_ms) * 100
                          )}%`,
                        }}
                      />
                    </div>
                  </div>
                )}
                {/* Abort button in modal */}
                <button
                  onClick={sendAbort}
                  className="mt-6 w-full py-3 bg-red-500/20 hover:bg-red-500/30 text-red-400 rounded-xl font-medium transition-colors border border-red-500/30"
                >
                  Abort Transaction
                </button>
              </>
            )}

            {state.appState === "SUCCESS" && (
              <>
                <div className="w-16 h-16 rounded-full bg-green-500/20 flex items-center justify-center text-green-500 mb-6">
                  <CheckCircle2 className="w-8 h-8" />
                </div>
                <h3 className="text-xl font-bold mb-2">Approved</h3>
                <p className="text-zinc-400 mb-6">
                  Auth Code:{" "}
                  <span className="text-white font-mono bg-zinc-800 px-2 py-1 rounded">
                    {state.lastResult?.ApprovalNo}
                  </span>
                </p>

                <div className="w-full bg-zinc-900 rounded-xl p-4 text-left space-y-2 mb-6 text-sm">
                  <div className="flex justify-between">
                    <span className="text-zinc-500">Amount</span>
                    <span className="font-mono">
                      $
                      {(
                        parseInt(state.lastResult?.Amount || "0") / 100
                      ).toFixed(2)}
                    </span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-zinc-500">Order No</span>
                    <span className="font-mono text-xs">
                      {state.lastResult?.OrderNo}
                    </span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-zinc-500">Card</span>
                    <span className="font-mono text-xs">
                      {state.lastResult?.CardNo}
                    </span>
                  </div>
                </div>

                <button
                  onClick={handleDismiss}
                  className="w-full py-3 bg-zinc-800 hover:bg-zinc-700 rounded-xl font-medium transition-colors"
                >
                  Close
                </button>
              </>
            )}

            {(state.appState === "ERROR" || state.appState === "TIMEOUT") && (
              <>
                <div className="w-16 h-16 rounded-full bg-red-500/20 flex items-center justify-center text-red-500 mb-6">
                  <AlertTriangle className="w-8 h-8" />
                </div>
                <h3 className="text-xl font-bold mb-2">
                  {state.appState === "TIMEOUT"
                    ? "Transaction Timeout"
                    : "Transaction Failed"}
                </h3>
                <p className="text-red-400 mb-6">{state.message}</p>
                <button
                  onClick={handleDismiss}
                  className="w-full py-3 bg-zinc-800 hover:bg-zinc-700 rounded-xl font-medium transition-colors"
                >
                  Dismiss
                </button>
              </>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

export default App;
