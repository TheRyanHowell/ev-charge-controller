"use client";

import type { ReactNode } from "react";

import { useEffect, useRef, useCallback } from "react";

interface DialogProps {
  isOpen: boolean;
  onClose: () => void;
  children: ReactNode;
  "aria-labelledby"?: string;
}

export default function Dialog({
  isOpen,
  onClose,
  children,
  "aria-labelledby": labelledBy,
}: DialogProps) {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const onCloseRef = useRef(onClose);
  const previousFocusRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    onCloseRef.current = onClose;
  }, [onClose]);

  // Sync dialog visibility with isOpen
  useEffect(() => {
    const dialog = dialogRef.current;
    if (!dialog) return;
    if (isOpen) {
      previousFocusRef.current = document.activeElement as HTMLElement;
      dialog.showModal();
    } else {
      dialog.close();
      previousFocusRef.current?.focus();
    }
  }, [isOpen]);

  // Handle native close event (Escape key, dialog.close())
  const handleClose = useCallback(() => {
    onCloseRef.current();
  }, []);

  // Handle backdrop click (clicking outside dialog content)
  const handleBackdropClick = useCallback(
    (e: React.MouseEvent<HTMLDialogElement>) => {
      if (e.target === e.currentTarget) {
        onCloseRef.current();
      }
    },
    [],
  );

  return (
    <dialog
      ref={dialogRef}
      className="z-50 m-0 h-screen w-screen max-h-none bg-transparent p-0 border-0 overflow-visible [&::backdrop]:bg-black/60"
      onClose={handleClose}
      onClick={handleBackdropClick}
      aria-labelledby={labelledBy}
    >
      <div className="flex items-center justify-center w-screen h-screen">
        {children}
      </div>
    </dialog>
  );
}
