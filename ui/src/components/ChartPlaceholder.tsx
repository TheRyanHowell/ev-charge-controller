interface ChartPlaceholderProps {
  message: string;
  heightPx: number;
}

export function ChartPlaceholder({ message, heightPx }: ChartPlaceholderProps) {
  return (
    <div className="mt-4 sm:mt-6 rounded-lg">
      <div
        className="flex items-center justify-center p-4"
        style={{ minHeight: `${heightPx}px` }}
      >
        <p className="text-xs text-gray-500 italic">{message}</p>
      </div>
    </div>
  );
}
