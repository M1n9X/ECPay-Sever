import { useState, useEffect, useRef } from "react";
import { usePOS } from "./hooks/usePOS";
import { useOrders } from "./hooks/useOrders";
import type { Order } from "./hooks/useOrders";
import { Keypad } from "./components/Keypad";
import { OrderHistory } from "./components/OrderHistory";
import {
  CreditCard,
  Terminal,
  CheckCircle2,
  RotateCcw,
  AlertTriangle,
  History,
  Wifi,
  WifiOff,
} from "lucide-react";
import { clsx } from "clsx";

function App() {
  const { connected, status, message, lastResult, sendCommand, reset } =
    usePOS();
  const { orders, addOrder, markRefunded } = useOrders();
  const [tab, setTab] = useState<"SALE" | "REFUND">("SALE");
  const [amount, setAmount] = useState("");
  const [orderNo, setOrderNo] = useState("");
  const [refundingOrderId, setRefundingOrderId] = useState<string | null>(null);

  const handleInput = (val: string) => {
    if (amount.length > 8) return;
    setAmount((prev) => prev + val);
  };

  const handleClear = () => setAmount("");
  const handleDelete = () => setAmount((prev) => prev.slice(0, -1));

  const handleCheckout = () => {
    if (!amount || parseInt(amount) === 0) return;
    sendCommand(tab, amount, tab === "REFUND" ? orderNo : undefined);
  };

  const handleRefundOrder = (order: Order) => {
    setTab("REFUND");
    setAmount(order.amount.toString());
    setOrderNo(order.orderNo);
    setRefundingOrderId(order.id);
  };
  // Track which transaction we've already processed to avoid duplicates
  const processedApprovalRef = useRef<string | null>(null);
  const refundingOrderIdRef = useRef<string | null>(null);

  // Sync ref with state for refundingOrderId
  useEffect(() => {
    refundingOrderIdRef.current = refundingOrderId;
  }, [refundingOrderId]);

  // Save order when transaction succeeds
  useEffect(() => {
    if (status === "SUCCESS" && lastResult) {
      // Skip if we already processed this exact transaction
      const currentApproval = lastResult.ApprovalNo;
      if (processedApprovalRef.current === currentApproval) {
        return;
      }
      processedApprovalRef.current = currentApproval;

      const orderData = {
        type: (lastResult.TransType === "01" ? "SALE" : "REFUND") as
          | "SALE"
          | "REFUND",
        amount: parseInt(lastResult.Amount || "0"),
        orderNo: lastResult.OrderNo || "",
        approvalNo: lastResult.ApprovalNo || "",
        cardNo: lastResult.CardNo || "",
      };
      addOrder(orderData);

      // If this was a refund for an existing order, mark it
      if (refundingOrderIdRef.current && orderData.type === "REFUND") {
        markRefunded(refundingOrderIdRef.current);
        setRefundingOrderId(null);
      }
    }
  }, [status, lastResult, addOrder, markRefunded]);

  return (
    <div className="h-screen bg-black text-white flex flex-col lg:flex-row items-center lg:items-start justify-center gap-4 p-3 lg:p-4 overflow-hidden">
      {/* Connection Status Header */}
      <div className="fixed top-4 left-4 right-4 flex items-center justify-between z-40">
        <div className="flex items-center gap-3 bg-zinc-900/80 backdrop-blur-md px-4 py-2 rounded-full border border-zinc-800">
          {connected ? (
            <Wifi className="w-4 h-4 text-green-500" />
          ) : (
            <WifiOff className="w-4 h-4 text-red-500" />
          )}
          <span className="text-sm font-medium text-zinc-400">
            {connected ? "Server Connected" : "Disconnected"}
          </span>
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
          <div className="flex p-1 bg-surface rounded-xl mb-4 border border-zinc-800">
            <button
              onClick={() => {
                setTab("SALE");
                setOrderNo("");
                setRefundingOrderId(null);
              }}
              className={clsx(
                "flex-1 flex items-center justify-center gap-2 py-2.5 rounded-lg text-sm font-medium transition-all",
                tab === "SALE"
                  ? "bg-zinc-800 text-white shadow-lg"
                  : "text-zinc-500 hover:text-zinc-300"
              )}
            >
              <CreditCard className="w-4 h-4" />
              Sale
            </button>
            <button
              onClick={() => setTab("REFUND")}
              className={clsx(
                "flex-1 flex items-center justify-center gap-2 py-2.5 rounded-lg text-sm font-medium transition-all",
                tab === "REFUND"
                  ? "bg-zinc-800 text-white shadow-lg"
                  : "text-zinc-500 hover:text-zinc-300"
              )}
            >
              <RotateCcw className="w-4 h-4" />
              Refund
            </button>
          </div>

          {/* Refund Order Input */}
          {tab === "REFUND" && (
            <div className="mb-3">
              <label className="text-xs font-bold text-zinc-500 uppercase tracking-widest pl-1">
                Original Order No
              </label>
              <input
                type="text"
                value={orderNo}
                onChange={(e) => setOrderNo(e.target.value)}
                placeholder="Enter Order No"
                className="w-full mt-1 bg-surface border border-zinc-800 rounded-xl px-4 py-2.5 text-white focus:outline-none focus:ring-2 focus:ring-blue-500/50 transition-all font-mono text-sm"
              />
            </div>
          )}

          {/* Keypad */}
          <Keypad
            onInput={handleInput}
            onClear={handleClear}
            onDelete={handleDelete}
            amount={amount}
          />

          {/* Action Button */}
          <button
            onClick={handleCheckout}
            disabled={!connected || !amount || (tab === "REFUND" && !orderNo)}
            className="w-full mt-4 py-3.5 rounded-xl bg-gradient-to-r from-blue-600 to-indigo-600 font-bold text-lg shadow-lg shadow-blue-500/20 hover:scale-[1.02] active:scale-[0.98] transition-all disabled:opacity-50 disabled:pointer-events-none"
          >
            {tab === "SALE" ? "Charge" : "Refund"} $
            {(parseInt(amount || "0") / 100).toFixed(2)}
          </button>
        </div>
      </div>

      {/* Order History Panel */}
      <div className="w-full max-w-md lg:w-80 mt-2 lg:mt-12">
        <div className="bg-zinc-950 border border-zinc-800 rounded-xl p-4 shadow-xl">
          <div className="flex items-center gap-2 mb-3">
            <History className="w-4 h-4 text-zinc-500" />
            <h2 className="text-sm font-bold text-zinc-400 uppercase tracking-widest">
              Recent Transactions
            </h2>
          </div>
          <OrderHistory orders={orders} onRefund={handleRefundOrder} />
        </div>
      </div>

      {/* Status Overlay Modal */}
      {status !== "IDLE" && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-sm p-4">
          <div className="bg-zinc-900 border border-zinc-800 rounded-3xl p-8 max-w-sm w-full shadow-2xl flex flex-col items-center text-center">
            {status === "PROCESSING" && (
              <>
                <div className="w-16 h-16 rounded-full border-4 border-blue-500/30 border-t-blue-500 animate-spin mb-6" />
                <h3 className="text-xl font-bold mb-2">Processing</h3>
                <p className="text-zinc-400">{message}</p>
              </>
            )}

            {status === "SUCCESS" && (
              <>
                <div className="w-16 h-16 rounded-full bg-green-500/20 flex items-center justify-center text-green-500 mb-6">
                  <CheckCircle2 className="w-8 h-8" />
                </div>
                <h3 className="text-xl font-bold mb-2">Approved</h3>
                <p className="text-zinc-400 mb-6">
                  Auth Code:{" "}
                  <span className="text-white font-mono bg-zinc-800 px-2 py-1 rounded">
                    {lastResult?.ApprovalNo}
                  </span>
                </p>

                <div className="w-full bg-surface rounded-xl p-4 text-left space-y-2 mb-6 text-sm">
                  <div className="flex justify-between">
                    <span className="text-zinc-500">Amount</span>
                    <span className="font-mono">
                      ${(parseInt(lastResult?.Amount || "0") / 100).toFixed(2)}
                    </span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-zinc-500">Order No</span>
                    <span className="font-mono text-xs">
                      {lastResult?.OrderNo}
                    </span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-zinc-500">Card</span>
                    <span className="font-mono text-xs">
                      {lastResult?.CardNo}
                    </span>
                  </div>
                </div>

                <button
                  onClick={() => {
                    reset();
                    setAmount("");
                    setOrderNo("");
                  }}
                  className="w-full py-3 bg-zinc-800 hover:bg-zinc-700 rounded-xl font-medium transition-colors"
                >
                  Close
                </button>
              </>
            )}

            {status === "FAIL" && (
              <>
                <div className="w-16 h-16 rounded-full bg-red-500/20 flex items-center justify-center text-red-500 mb-6">
                  <AlertTriangle className="w-8 h-8" />
                </div>
                <h3 className="text-xl font-bold mb-2">Transaction Failed</h3>
                <p className="text-red-400 mb-6">{message}</p>
                <button
                  onClick={reset}
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
