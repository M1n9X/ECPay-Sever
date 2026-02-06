import type { Order } from "../hooks/useOrders";
import { clsx } from "clsx";
import { RefreshCw, CheckCircle, Clock } from "lucide-react";

interface OrderHistoryProps {
  orders: Order[];
  onRefund: (order: Order) => void;
}

export const OrderHistory = ({ orders, onRefund }: OrderHistoryProps) => {
  if (orders.length === 0) {
    return (
      <div className="text-center py-8 text-zinc-500">
        <Clock className="w-8 h-8 mx-auto mb-2 opacity-50" />
        <p className="text-sm">No transactions yet</p>
      </div>
    );
  }

  return (
    <div className="space-y-2 max-h-64 overflow-y-auto">
      {orders.map((order) => (
        <div
          key={order.id}
          className={clsx(
            "p-3 rounded-xl border transition-all",
            order.type === "REFUND"
              ? "bg-orange-500/5 border-orange-500/20"
              : order.refunded
              ? "bg-zinc-800/50 border-zinc-700 opacity-60"
              : "bg-surface border-zinc-800 hover:border-zinc-700"
          )}
        >
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              {order.refunded ? (
                <CheckCircle className="w-4 h-4 text-zinc-500" />
              ) : order.type === "REFUND" ? (
                <RefreshCw className="w-4 h-4 text-orange-500" />
              ) : (
                <div className="w-2 h-2 rounded-full bg-green-500" />
              )}
              <span className="font-mono text-sm">
                ${(order.amount / 100).toFixed(2)}
              </span>
              <span
                className={clsx(
                  "text-xs px-2 py-0.5 rounded-full",
                  order.type === "REFUND"
                    ? "bg-orange-500/20 text-orange-400"
                    : "bg-green-500/20 text-green-400"
                )}
              >
                {order.type}
              </span>
            </div>
            <div className="flex items-center gap-2">
              <span className="text-xs text-zinc-500 font-mono">
                {order.approvalNo}
              </span>
              {order.type === "SALE" && !order.refunded && (
                <button
                  onClick={() => onRefund(order)}
                  className="text-xs px-2 py-1 rounded-lg bg-red-500/10 text-red-400 hover:bg-red-500/20 transition-colors"
                >
                  Refund
                </button>
              )}
            </div>
          </div>
          <div className="mt-1 flex justify-between text-xs text-zinc-500">
            <span className="font-mono">{order.cardNo}</span>
            <span>{order.timestamp.toLocaleTimeString()}</span>
          </div>
        </div>
      ))}
    </div>
  );
};
