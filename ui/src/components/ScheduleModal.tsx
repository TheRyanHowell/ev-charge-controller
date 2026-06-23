"use client";

import type { SchedulePayload } from "@/hooks/useSchedule";
import type { Schedule } from "@/lib/schemas";

import Dialog from "@/components/Dialog";
import ScheduleForm from "@/components/ScheduleForm";
import { useId } from "react";

interface ScheduleModalProps {
  isOpen: boolean;
  onClose: () => void;
  schedule: Schedule | null | undefined;
  onSave: (payload: SchedulePayload) => void;
  isSaving?: boolean;
}

export default function ScheduleModal({
  isOpen,
  onClose,
  schedule,
  onSave,
  isSaving,
}: ScheduleModalProps) {
  const titleId = useId();

  if (!isOpen) return null;

  return (
    <Dialog isOpen onClose={onClose} aria-labelledby={titleId}>
      <div className="w-[420px] max-w-full mx-4 bg-gray-900 rounded-xl shadow-2xl overflow-hidden">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-700">
          <h2 id={titleId} className="text-lg font-semibold text-white">
            Charge Schedule
          </h2>
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-white transition-colors rounded-lg p-1.5
              hover:bg-gray-700/50 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
            aria-label="Close schedule settings"
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
          <ScheduleForm
            schedule={schedule}
            onSave={(payload) => {
              onSave(payload);
              onClose();
            }}
            isSaving={isSaving}
            onSkip={onClose}
            saveLabel="Save"
          />
        </div>
      </div>
    </Dialog>
  );
}
