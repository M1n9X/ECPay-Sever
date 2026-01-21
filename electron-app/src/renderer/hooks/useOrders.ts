import { useState, useCallback } from "react";

export interface Order {
  id: string;
  type: "SALE" | "REFUND";
  amount: number;
  orderNo: string;
  approvalNo: string;
  cardNo: string;
  timestamp: Date;
  refunded: boolean;
}

export const useOrders = () => {
  const [orders, setOrders] = useState<Order[]>([]);

  const addOrder = useCallback(
    (order: Omit<Order, "id" | "timestamp" | "refunded">) => {
      const newOrder: Order = {
        ...order,
        id: `${Date.now()}-${Math.random().toString(36).substring(2, 9)}`,
        timestamp: new Date(),
        refunded: order.type === "REFUND",
      };
      setOrders((prev) => [newOrder, ...prev]);
      return newOrder;
    },
    []
  );

  const markRefunded = useCallback((orderId: string) => {
    setOrders((prev) =>
      prev.map((order) =>
        order.id === orderId ? { ...order, refunded: true } : order
      )
    );
  }, []);

  const getRefundableOrders = useCallback(() => {
    return orders.filter((o) => o.type === "SALE" && !o.refunded);
  }, [orders]);

  return {
    orders,
    addOrder,
    markRefunded,
    getRefundableOrders,
  };
};
