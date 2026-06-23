"use client";

import { useState, useCallback } from "react";

export default function ConsoleCommandsBlock({
  commands,
}: {
  commands: string;
}) {
  const [copied, setCopied] = useState(false);

  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(commands);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // clipboard API may not be available
    }
  }, [commands]);

  return (
    <div className="relative">
      <pre className="rounded bg-gray-900 border border-gray-700 px-3 py-2 text-xs font-mono text-gray-300 whitespace-pre-wrap break-all">
        {commands}
      </pre>
      <button
        type="button"
        onClick={() => void handleCopy()}
        className="absolute top-2 right-2 rounded bg-gray-700 px-2 py-1 text-xs text-gray-300 hover:bg-gray-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 transition-colors"
      >
        {copied ? "Copied!" : "Copy"}
      </button>
    </div>
  );
}
