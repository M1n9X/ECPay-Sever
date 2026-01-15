import { Delete, Eraser } from "lucide-react";
import { clsx } from "clsx";

interface KeypadProps {
  onInput: (val: string) => void;
  onClear: () => void;
  onDelete: () => void;
  amount: string;
}

export const Keypad = ({ onInput, onClear, onDelete, amount }: KeypadProps) => {
  const keys = ["1", "2", "3", "4", "5", "6", "7", "8", "9", "00", "0"];

  return (
    <div className="grid grid-cols-3 gap-3 w-full max-w-sm mx-auto">
      {/* Amount Display */}
      <div className="col-span-3 mb-4 p-6 bg-surface rounded-2xl border border-zinc-800 text-right">
        <span className="text-zinc-500 text-2xl mr-2">$</span>
        <span
          className={clsx(
            "text-4xl font-mono font-bold tracking-wider",
            !amount && "text-zinc-600"
          )}
        >
          {amount ? (parseInt(amount) / 100).toFixed(2) : "0.00"}
        </span>
      </div>

      {keys.map((k) => (
        <button
          key={k}
          onClick={() => onInput(k)}
          className="h-16 rounded-xl bg-zinc-900 border border-zinc-800 hover:bg-zinc-800 active:scale-95 transition-all text-xl font-bold text-white shadow-lg shadow-black/20"
        >
          {k}
        </button>
      ))}

      {/* Action Keys */}
      <button
        onClick={onDelete}
        className="h-16 rounded-xl bg-zinc-900 border border-zinc-800 hover:bg-zinc-800 active:scale-95 transition-all flex items-center justify-center text-white"
      >
        <Delete className="w-6 h-6" />
      </button>

      <button
        onClick={onClear}
        className="col-span-3 h-14 mt-2 rounded-xl bg-red-500/10 border border-red-500/20 text-red-500 hover:bg-red-500/20 uppercase font-bold tracking-widest text-sm"
      >
        Clear
      </button>
    </div>
  );
};
