"use client";

interface ToggleProps {
  checked: boolean;
  onChange: (checked: boolean) => void;
  disabled?: boolean;
  label?: string;
}

export default function Toggle({
  checked,
  onChange,
  disabled,
  label,
}: ToggleProps) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={`relative inline-flex h-6 w-11 shrink-0 items-center p-0.5
        cursor-pointer rounded-full border-0
        transition-colors duration-200 ease-in-out focus-visible:outline-none
        focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2
        focus-visible:ring-offset-gray-900
        ${checked ? "bg-blue-500" : "bg-gray-600"}
        ${disabled ? "opacity-50 cursor-not-allowed" : ""}`}
    >
      <span
        className={`pointer-events-none inline-block h-5 w-5 rounded-full
          bg-white shadow ring-0 transition duration-200 ease-in-out
          ${checked ? "translate-x-5" : "translate-x-0"}`}
      />
    </button>
  );
}
