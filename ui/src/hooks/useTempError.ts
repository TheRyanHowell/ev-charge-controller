import { TEMP_ERROR_FLASH_MS } from "@/lib/constants";
import { useState, useEffect, useCallback, useRef } from "react";

export function useTempError(timeoutMs = TEMP_ERROR_FLASH_MS) {
  const [error, setError] = useState<string | null>(null);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const flash = useCallback(
    (msg: string) => {
      if (timerRef.current) {
        clearTimeout(timerRef.current);
      }
      setError(msg);
      timerRef.current = setTimeout(() => setError(null), timeoutMs);
    },
    [timeoutMs],
  );

  useEffect(() => {
    return () => {
      if (timerRef.current) {
        clearTimeout(timerRef.current);
      }
    };
  }, []);

  return { error, flash };
}
