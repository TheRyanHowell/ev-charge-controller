"use client";

import type { TariffSettings } from "@/lib/schemas";

import { useTariff } from "@/hooks/useTariff";
import { useCallback, useEffect, useId, useRef, useState } from "react";

interface WindowDraft {
  start: string;
  end: string;
  rate: string;
}

const timePattern = /^([01]\d|2[0-3]):[0-5]\d$/;

function isTariff(settings: unknown): settings is TariffSettings {
  return (
    settings != null &&
    typeof (settings as TariffSettings).baseRatePence === "number" &&
    Array.isArray((settings as TariffSettings).offPeakWindows)
  );
}

function toDraftWindows(settings: TariffSettings): WindowDraft[] {
  return settings.offPeakWindows.map((w) => ({
    start: w.start,
    end: w.end,
    rate: String(w.ratePence),
  }));
}

/**
 * TariffSettingsSection lets the user configure their electricity tariff: a base
 * (peak) rate plus zero or more off-peak windows, each with its own rate. Rates
 * are entered in pence per kWh. Changes save automatically on blur or on
 * add/remove of a window.
 */
export default function TariffSettingsSection() {
  const { settings, isLoading, updateTariff } = useTariff();
  const baseId = useId();

  const [baseRate, setBaseRate] = useState("");
  const [windows, setWindows] = useState<WindowDraft[]>([]);
  const [error, setError] = useState<string | null>(null);

  // Refs mirror state so onBlur handlers always read the latest value even
  // before React has re-rendered (state updates are async; refs are sync).
  const latestBaseRate = useRef(baseRate);
  const latestWindows = useRef(windows);

  // Seed editable draft from the loaded tariff without an effect.
  const [seededFrom, setSeededFrom] = useState<TariffSettings | null>(null);
  if (isTariff(settings) && settings !== seededFrom) {
    setSeededFrom(settings);
    const rate = String(settings.baseRatePence);
    const wins = toDraftWindows(settings);
    setBaseRate(rate);
    setWindows(wins);
  }

  // Keep refs in sync with the seeded values so onBlur handlers can read the
  // current tariff even before React has re-rendered after the seed.
  useEffect(() => {
    if (!seededFrom) return;
    latestBaseRate.current = String(seededFrom.baseRatePence);
    latestWindows.current = toDraftWindows(seededFrom);
  }, [seededFrom]);

  const autoSave = useCallback(
    async (rate: string, wins: WindowDraft[]) => {
      setError(null);
      const base = Number(rate);
      if (!Number.isFinite(base) || base < 0) {
        setError("Base rate must be a number of pence (0 or more).");
        return;
      }
      const parsedWindows = [];
      for (const [i, w] of wins.entries()) {
        const r = Number(w.rate);
        if (!timePattern.test(w.start) || !timePattern.test(w.end)) {
          setError(`Window ${i + 1}: valid start and end time required.`);
          return;
        }
        if (w.start === w.end) {
          setError(`Window ${i + 1}: start and end must differ.`);
          return;
        }
        if (!Number.isFinite(r) || r < 0) {
          setError(`Window ${i + 1}: rate must be 0 or more pence.`);
          return;
        }
        parsedWindows.push({ start: w.start, end: w.end, ratePence: r });
      }
      try {
        await updateTariff({
          baseRatePence: base,
          offPeakWindows: parsedWindows,
        });
      } catch (e) {
        setError(e instanceof Error ? e.message : "Failed to save tariff.");
      }
    },
    [updateTariff],
  );

  const addWindow = useCallback(() => {
    // Default rate matches current base rate so the user can tweak it.
    const newWindow: WindowDraft = {
      start: "00:30",
      end: "04:30",
      rate: latestBaseRate.current,
    };
    const next = [...latestWindows.current, newWindow];
    latestWindows.current = next;
    setWindows(next);
    void autoSave(latestBaseRate.current, next);
  }, [autoSave]);

  const removeWindow = useCallback(
    (index: number) => {
      const next = latestWindows.current.filter((_, i) => i !== index);
      latestWindows.current = next;
      setWindows(next);
      void autoSave(latestBaseRate.current, next);
    },
    [autoSave],
  );

  const updateWindow = useCallback(
    (index: number, patch: Partial<WindowDraft>) => {
      setWindows((prev) => {
        const next = prev.map((w, i) => (i === index ? { ...w, ...patch } : w));
        return next;
      });
    },
    [],
  );

  /**
   * Reads all window values directly from the DOM. This is used in onBlur
   * handlers to guarantee we capture the current input value even when the
   * onChange handler hasn't run yet (e.g. Playwright's fill() dispatches
   * events asynchronously relative to blur).
   */
  const readWindowsFromDom = useCallback((): WindowDraft[] => {
    const partials: Partial<WindowDraft>[] = [];
    document
      .querySelectorAll("[aria-label^='Off-peak window']")
      .forEach((el) => {
        const input =
          el instanceof HTMLInputElement ? el : el.querySelector("input");
        if (!(input instanceof HTMLInputElement)) return;
        const label = input.getAttribute("aria-label") || "";
        const match = label.match(/Off-peak window (\d+)/);
        if (!match) return;
        const index = parseInt(match[1] ?? "0", 10) - 1;
        if (label.endsWith("start")) {
          while (partials.length <= index) partials.push({});
          partials[index] = { ...partials[index], start: input.value };
        } else if (label.endsWith("end")) {
          while (partials.length <= index) partials.push({});
          partials[index] = { ...partials[index], end: input.value };
        } else if (label.endsWith("rate")) {
          while (partials.length <= index) partials.push({});
          partials[index] = { ...partials[index], rate: input.value };
        }
      });
    return partials.filter(
      (w): w is WindowDraft =>
        w.start != null && w.end != null && w.rate != null,
    );
  }, []);

  return (
    <section className="space-y-3" aria-labelledby="tariff-heading">
      <div>
        <p
          id="tariff-heading"
          className="text-xs font-medium text-gray-400 uppercase tracking-wide"
        >
          Electricity tariff
        </p>
        <p className="text-xs text-gray-400 mt-0.5">
          Used to calculate charging costs. Add off-peak windows for cheaper
          overnight rates.
        </p>
      </div>

      {isLoading ? (
        <p className="text-xs text-gray-400">Loading tariff…</p>
      ) : (
        <>
          <div>
            <label
              htmlFor={baseId}
              className="block text-xs text-gray-400 mb-1"
            >
              Base rate (pence per kWh)
            </label>
            <input
              id={baseId}
              type="number"
              inputMode="decimal"
              min={0}
              step="0.01"
              value={baseRate}
              onChange={(e) => {
                latestBaseRate.current = e.target.value;
                setBaseRate(e.target.value);
              }}
              onBlur={() =>
                void autoSave(latestBaseRate.current, readWindowsFromDom())
              }
              className="w-full rounded bg-gray-900 border border-gray-700 px-2.5 py-1.5 text-sm text-white placeholder-gray-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
            />
          </div>

          <div className="space-y-2">
            <p className="text-xs font-medium text-gray-300">
              Off-peak windows
            </p>
            {windows.length === 0 && (
              <p className="text-xs text-gray-500">
                No off-peak windows. All energy is billed at the base rate.
              </p>
            )}
            <ul className="space-y-2">
              {windows.map((w, i) => (
                <li
                  key={i}
                  className="flex flex-wrap items-end gap-2 rounded-lg border border-gray-700 bg-gray-800/40 p-2"
                >
                  <label className="flex flex-col text-xs text-gray-400">
                    <span className="mb-1">Start</span>
                    <input
                      type="time"
                      aria-label={`Off-peak window ${i + 1} start`}
                      value={w.start}
                      onChange={(e) =>
                        updateWindow(i, { start: e.target.value })
                      }
                      onBlur={() =>
                        void autoSave(
                          latestBaseRate.current,
                          readWindowsFromDom(),
                        )
                      }
                      className="rounded bg-gray-900 border border-gray-700 px-2 py-1 text-sm text-white focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
                    />
                  </label>
                  <label className="flex flex-col text-xs text-gray-400">
                    <span className="mb-1">End</span>
                    <input
                      type="time"
                      aria-label={`Off-peak window ${i + 1} end`}
                      value={w.end}
                      onChange={(e) => updateWindow(i, { end: e.target.value })}
                      onBlur={() =>
                        void autoSave(
                          latestBaseRate.current,
                          readWindowsFromDom(),
                        )
                      }
                      className="rounded bg-gray-900 border border-gray-700 px-2 py-1 text-sm text-white focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
                    />
                  </label>
                  <label className="flex flex-1 flex-col text-xs text-gray-400">
                    <span className="mb-1">Rate (p/kWh)</span>
                    <input
                      type="number"
                      inputMode="decimal"
                      min={0}
                      step="0.01"
                      aria-label={`Off-peak window ${i + 1} rate`}
                      value={w.rate}
                      onChange={(e) =>
                        updateWindow(i, { rate: e.target.value })
                      }
                      onBlur={() =>
                        void autoSave(
                          latestBaseRate.current,
                          readWindowsFromDom(),
                        )
                      }
                      className="w-full rounded bg-gray-900 border border-gray-700 px-2 py-1 text-sm text-white focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
                    />
                  </label>
                  <button
                    type="button"
                    onClick={() => removeWindow(i)}
                    aria-label={`Remove off-peak window ${i + 1}`}
                    className="rounded px-2 py-1 text-xs text-gray-400 hover:text-red-400 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-red-500 transition-colors"
                  >
                    <i className="fa-solid fa-trash-can" aria-hidden="true" />
                  </button>
                </li>
              ))}
            </ul>
            <button
              type="button"
              onClick={addWindow}
              className="text-xs text-blue-400 hover:text-blue-300 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-blue-500 rounded"
            >
              <i className="fa-solid fa-plus mr-1" aria-hidden="true" />
              Add off-peak window
            </button>
          </div>

          {error && (
            <p role="alert" className="text-xs text-red-400">
              {error}
            </p>
          )}
        </>
      )}
    </section>
  );
}
