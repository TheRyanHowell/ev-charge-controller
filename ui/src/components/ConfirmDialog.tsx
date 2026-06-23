"use client";

import Dialog from "@/components/Dialog";
import { useEffect, useRef } from "react";

interface ConfirmDialogProps {
  title: string;
  message: string;
  onConfirm: () => void;
  onCancel: () => void;
  confirmLabel?: string;
  cancelLabel?: string;
  variant?: "danger" | "info";
}

export default function ConfirmDialog({
  title,
  message,
  onConfirm,
  onCancel,
  confirmLabel = "Confirm",
  cancelLabel = "Cancel",
  variant = "danger",
}: ConfirmDialogProps) {
  const confirmBtnRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    confirmBtnRef.current?.focus();
  }, []);

  const confirmClasses =
    variant === "danger"
      ? "bg-red-600 hover:bg-red-500 focus-visible:ring-red-500"
      : "bg-blue-600 hover:bg-blue-500 focus-visible:ring-blue-500";

  return (
    <Dialog isOpen onClose={onCancel} aria-labelledby="confirm-dialog-title">
      <div className="w-[480px] max-w-full mx-4 bg-gray-900 rounded-xl shadow-2xl overflow-hidden">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-700">
          <h2
            id="confirm-dialog-title"
            className="text-lg font-semibold text-white"
          >
            {title}
          </h2>
          <button
            onClick={onCancel}
            className="text-gray-400 hover:text-white transition-colors
              rounded-lg p-1.5 hover:bg-gray-700/50 focus:outline-none
              focus-visible:ring-2 focus-visible:ring-blue-500"
            aria-label="Close dialog"
          >
            <svg
              width="16"
              height="16"
              viewBox="0 0 16 16"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
            >
              <path d="M12 4L4 12M4 4l8 8" />
            </svg>
          </button>
        </div>

        {/* Body */}
        <div className="px-6 py-5">
          <p className="text-sm text-gray-400">{message}</p>
        </div>

        {/* Footer */}
        <div
          className="px-6 py-4 border-t border-gray-700 bg-gray-800/50 flex items-center justify-end gap-3"
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              onConfirm();
            }
          }}
        >
          <button
            type="button"
            onClick={onCancel}
            className="px-4 py-2 text-sm font-medium text-gray-300
              hover:text-white rounded-lg hover:bg-gray-700 transition-colors
              focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
          >
            {cancelLabel}
          </button>
          <button
            ref={confirmBtnRef}
            type="button"
            onClick={onConfirm}
            className={`px-4 py-2 text-sm font-medium text-white rounded-lg
              focus:outline-none focus-visible:ring-2 ${confirmClasses}
              transition-colors`}
          >
            {confirmLabel}
          </button>
        </div>
      </div>
    </Dialog>
  );
}
