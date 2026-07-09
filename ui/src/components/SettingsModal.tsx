"use client";

import type { Plug, Vehicle } from "@/lib/schemas";

import ConsoleCommandsBlock from "@/components/ConsoleCommandsBlock";
import Dialog from "@/components/Dialog";
import TariffSettingsSection from "@/components/TariffSettingsSection";
import Toggle from "@/components/Toggle";
import { useFocusOnMount } from "@/hooks/useFocusOnMount";
import { apiPost } from "@/lib/api";
import {
  isPushEnabled,
  subscribeToPush,
  unsubscribeFromPush,
  registerServiceWorker,
  getPushSubscription,
} from "@/lib/push";
import { queryKeys } from "@/lib/queryKeys";
import { ConsoleCommandsResultSchema } from "@/lib/schemas";
import { useThemeStore } from "@/stores/themeStore";
import { useQueryClient } from "@tanstack/react-query";
import { useState, useEffect, useCallback, useId } from "react";

function AutoConfigureForm({
  plugId,
  onSuccess,
  onCancel,
}: {
  plugId: string;
  onSuccess: () => void;
  onCancel: () => void;
}) {
  const ipId = useId();
  const passId = useId();
  const [ip, setIp] = useState("");
  const [tasmotaPass, setTasmotaPass] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const handleSubmit = useCallback(async () => {
    if (!ip.trim()) return;
    setLoading(true);
    setError(null);
    try {
      await apiPost(
        `/api/plugs/${plugId}/configure`,
        ConsoleCommandsResultSchema,
        {
          tasmotaIP: ip,
          tasmotaPassword: tasmotaPass || undefined,
        },
      );
      onSuccess();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Something went wrong");
    } finally {
      setLoading(false);
    }
  }, [ip, tasmotaPass, plugId, onSuccess]);

  return (
    <div className="space-y-3 rounded-lg border border-dashed border-blue-500/30 bg-blue-500/5 p-3">
      <p className="text-xs font-medium text-accent-muted">Auto-configure</p>
      <p className="text-xs text-fg-muted">
        Push MQTT settings to the device. You&apos;ll need its IP address.
      </p>
      <div>
        <label htmlFor={ipId} className="block text-xs text-fg-muted mb-1">
          Plug IP address *
        </label>
        <input
          id={ipId}
          type="text"
          value={ip}
          onChange={(e) => setIp(e.target.value)}
          placeholder="192.168.1.50"
          className="w-full rounded bg-surface-raised border border-border px-2.5 py-1.5 text-sm text-fg placeholder-fg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
        />
      </div>
      <div>
        <label htmlFor={passId} className="block text-xs text-fg-muted mb-1">
          Tasmota web admin password (optional)
        </label>
        <input
          id={passId}
          type="password"
          value={tasmotaPass}
          onChange={(e) => setTasmotaPass(e.target.value)}
          placeholder="Leave blank if none set"
          className="w-full rounded bg-surface-raised border border-border px-2.5 py-1.5 text-sm text-fg placeholder-fg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
        />
      </div>
      {error && <p className="text-xs text-danger">{error}</p>}
      <div className="flex gap-2">
        <button
          type="button"
          onClick={onCancel}
          disabled={loading}
          className="flex-1 rounded bg-surface px-3 py-1.5 text-xs text-fg-secondary hover:bg-surface-hover disabled:opacity-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 transition-colors"
        >
          Cancel
        </button>
        <button
          type="button"
          onClick={() => void handleSubmit()}
          disabled={!ip.trim() || loading}
          className="flex-1 rounded bg-blue-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-blue-500 disabled:opacity-40 disabled:cursor-not-allowed focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 transition-colors"
        >
          {loading ? "Configuring…" : "Configure →"}
        </button>
      </div>
    </div>
  );
}

interface SettingsModalProps {
  isOpen: boolean;
  onClose: () => void;
  plug: Plug | null;
  vehicles: Vehicle[];
  onUpdateName: (name: string) => void;
  onDelete: () => void;
  onUpdateNotificationPrefs?: (
    vehicleId: string,
    prefs: {
      notifyChargeStarted?: boolean;
      notifyChargeComplete?: boolean;
      notifyChargerOffline?: boolean;
      notifyMaintenanceOffline?: boolean;
    },
  ) => void;
  maintenancePlug?: Plug | null;
  onAdd12V?: () => void;
  onAddChargingPlug?: () => void;
  onDeleteMaintenance?: () => void;
  onUpdateMaintenanceName?: (name: string) => void;
}

export default function SettingsModal({
  isOpen,
  onClose,
  plug,
  vehicles,
  onUpdateName,
  onDelete,
  onUpdateNotificationPrefs,
  maintenancePlug,
  onAdd12V,
  onAddChargingPlug,
  onDeleteMaintenance,
  onUpdateMaintenanceName,
}: SettingsModalProps) {
  const theme = useThemeStore((state) => state.theme);
  const toggleTheme = useThemeStore((state) => state.toggleTheme);
  const [pushSubscribed, setPushSubscribed] = useState(false);
  const [pushLoading, setPushLoading] = useState(false);

  useEffect(() => {
    if (!isOpen) return;
    if (!isPushEnabled()) return;
    (async () => {
      const sub = await getPushSubscription();
      setPushSubscribed(!!sub);
    })();
  }, [isOpen]);

  const handlePushToggle = useCallback(async () => {
    setPushLoading(true);
    try {
      if (pushSubscribed) {
        await unsubscribeFromPush();
        setPushSubscribed(false);
      } else {
        await registerServiceWorker();
        const sub = await subscribeToPush();
        setPushSubscribed(!!sub);
      }
    } catch (e) {
      console.error("Push toggle failed:", e);
    } finally {
      setPushLoading(false);
    }
  }, [pushSubscribed]);

  const selectedVehicle =
    (plug?.vehicleId ? vehicles.find((v) => v.id === plug.vehicleId) : null) ??
    null;

  if (!isOpen) return null;

  return (
    <Dialog isOpen onClose={onClose} aria-labelledby="settings-title">
      {/* Widens to two-panel layout on sm+ screens */}
      <div className="w-full max-w-[480px] sm:max-w-[720px] mx-4 bg-surface-raised rounded-xl shadow-2xl overflow-hidden">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-border">
          <h2 id="settings-title" className="text-lg font-semibold text-fg">
            Settings
          </h2>
          <button
            onClick={onClose}
            className="text-fg-muted hover:text-fg transition-colors
              rounded-lg p-1.5 hover:bg-surface-hover/50 focus:outline-none
              focus-visible:ring-2 focus-visible:ring-blue-500"
            aria-label="Close settings"
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

        {/*
          Body: single scrollable column on mobile; side-by-side panels on sm+.
          Each panel gets its own max-height + scroll on wider screens.
        */}
        <div className="max-h-[70vh] overflow-y-auto sm:max-h-none sm:overflow-visible sm:grid sm:grid-cols-2 sm:divide-x sm:divide-border">
          {/* ── Left panel: General ───────────────────────────────────── */}
          <div className="px-6 py-5 space-y-5 sm:max-h-[70vh] sm:overflow-y-auto">
            <h3 className="text-sm font-medium text-fg">General</h3>

            <div className="flex items-center justify-between">
              <p className="text-xs text-fg-secondary">Dark mode</p>
              <Toggle
                checked={theme === "dark"}
                onChange={toggleTheme}
                label="Dark mode"
              />
            </div>

            {isPushEnabled() && (
              <>
                <div className="border-t border-border" />
                <div className="space-y-3">
                  <p className="text-xs font-medium text-fg-muted uppercase tracking-wide">
                    Push Notifications
                  </p>
                  <div className="flex items-center justify-between">
                    <p className="text-xs text-fg-secondary">
                      Enable device push notifications
                    </p>
                    <Toggle
                      checked={pushSubscribed}
                      onChange={() => handlePushToggle()}
                      disabled={pushLoading}
                      label="Enable device push notifications"
                    />
                  </div>
                </div>
              </>
            )}

            <div className="border-t border-border" />

            <TariffSettingsSection />
          </div>

          {/* ── Right panel: Vehicle settings ─────────────────────────── */}
          <div className="px-6 py-5 space-y-5 border-t border-border sm:border-t-0 sm:max-h-[70vh] sm:overflow-y-auto">
            <h3 className="text-sm font-medium text-fg">
              {selectedVehicle?.name ?? "Vehicle"}
            </h3>

            {/* Primary Charger */}
            <section className="space-y-3">
              <p className="text-xs font-medium text-fg-muted uppercase tracking-wide">
                Primary Charger
              </p>
              {plug ? (
                <PlugControls
                  plugId={plug.id}
                  name={plug.name}
                  online={plug.online}
                  deleteConfirmText="Delete this plug?"
                  onUpdateName={onUpdateName}
                  onDelete={() => {
                    onDelete();
                    onClose();
                  }}
                />
              ) : (
                <div className="flex items-center justify-between">
                  <span className="text-xs text-fg-muted">
                    No charging plug
                  </span>
                  {onAddChargingPlug && (
                    <button
                      type="button"
                      onClick={() => {
                        onAddChargingPlug();
                        onClose();
                      }}
                      className="text-xs text-accent-muted hover:text-accent focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-blue-500 rounded transition-colors"
                    >
                      Add charging plug →
                    </button>
                  )}
                </div>
              )}
            </section>

            <div className="border-t border-border" />

            {/* 12V Maintenance Charger */}
            <section className="space-y-3">
              <p className="text-xs font-medium text-fg-muted uppercase tracking-wide">
                12V Maintenance Charger
              </p>
              {maintenancePlug ? (
                <Maintenance12VSection
                  plug={maintenancePlug}
                  onDeleteMaintenance={onDeleteMaintenance}
                  onUpdateName={onUpdateMaintenanceName}
                />
              ) : (
                <div className="flex items-center justify-between">
                  <span className="text-xs text-fg-muted">No 12V charger</span>
                  {onAdd12V && (
                    <button
                      type="button"
                      onClick={() => {
                        onAdd12V();
                        onClose();
                      }}
                      className="text-xs text-accent-muted hover:text-accent focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-blue-500 rounded transition-colors"
                    >
                      Add 12V charger →
                    </button>
                  )}
                </div>
              )}
            </section>

            {/* Notifications */}
            {selectedVehicle && onUpdateNotificationPrefs && (
              <>
                <div className="border-t border-border" />
                <section className="space-y-3">
                  <p className="text-xs font-medium text-fg-muted uppercase tracking-wide">
                    Notifications
                  </p>
                  <div className="space-y-2">
                    <div className="flex items-center justify-between">
                      <span className="text-xs text-fg-secondary">
                        Charge started
                      </span>
                      <Toggle
                        checked={selectedVehicle.notifyChargeStarted}
                        onChange={(checked) =>
                          onUpdateNotificationPrefs(selectedVehicle.id, {
                            notifyChargeStarted: checked,
                          })
                        }
                        label="Charge started"
                      />
                    </div>
                    <div className="flex items-center justify-between">
                      <span className="text-xs text-fg-secondary">
                        Charge complete
                      </span>
                      <Toggle
                        checked={selectedVehicle.notifyChargeComplete}
                        onChange={(checked) =>
                          onUpdateNotificationPrefs(selectedVehicle.id, {
                            notifyChargeComplete: checked,
                          })
                        }
                        label="Charge complete"
                      />
                    </div>
                    <div className="flex items-center justify-between">
                      <span className="text-xs text-fg-secondary">
                        Charger offline
                      </span>
                      <Toggle
                        checked={selectedVehicle.notifyChargerOffline}
                        onChange={(checked) =>
                          onUpdateNotificationPrefs(selectedVehicle.id, {
                            notifyChargerOffline: checked,
                          })
                        }
                        label="Charger offline"
                      />
                    </div>
                    <div className="flex items-center justify-between">
                      <span className="text-xs text-fg-secondary">
                        12V maintenance charger offline
                      </span>
                      <Toggle
                        checked={selectedVehicle.notifyMaintenanceOffline}
                        onChange={(checked) =>
                          onUpdateNotificationPrefs(selectedVehicle.id, {
                            notifyMaintenanceOffline: checked,
                          })
                        }
                        label="12V maintenance charger offline"
                      />
                    </div>
                  </div>
                </section>
              </>
            )}
          </div>
        </div>

        {/* Footer */}
        <div className="px-6 py-4 border-t border-border bg-surface/50 flex items-center justify-end">
          <button
            onClick={onClose}
            className="px-4 py-2 text-sm font-medium text-fg-secondary
              hover:text-fg rounded-lg hover:bg-surface-hover transition-colors
              focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
          >
            Close
          </button>
        </div>
      </div>
    </Dialog>
  );
}

// ─── Shared plug controls (name edit, regen, reconfigure, delete) ────────────

function PlugControls({
  plugId,
  name,
  online,
  powerOn,
  deleteConfirmText,
  onUpdateName,
  onDelete,
}: {
  plugId: string;
  name: string;
  online: boolean;
  /** When provided, renders the 3-state maintenance dot (offline / on / off). */
  powerOn?: boolean;
  deleteConfirmText: string;
  onUpdateName?: (name: string) => void;
  onDelete?: () => void;
}) {
  const queryClient = useQueryClient();
  const nameInputId = useId();
  const [editingName, setEditingName] = useState(false);
  const [nameDraft, setNameDraft] = useState(name);
  const focusNameInput = useFocusOnMount<HTMLInputElement>();
  const [deleteConfirm, setDeleteConfirm] = useState(false);
  // null = idle; "path" = chooser; "auto" = AutoConfigureForm; "manual" = console commands
  const [configMode, setConfigMode] = useState<
    "path" | "auto" | "manual" | null
  >(null);
  const [regenLoading, setRegenLoading] = useState(false);
  const [regenError, setRegenError] = useState<string | null>(null);
  const [consoleCommands, setConsoleCommands] = useState<string | null>(null);

  // Status dot: 2-state for primary charger; 3-state for maintenance.
  const dotCls =
    powerOn === undefined
      ? online
        ? "bg-success"
        : "bg-fg-muted"
      : !online
        ? "bg-warning"
        : powerOn
          ? "bg-info"
          : "bg-fg-muted";
  const dotLabel =
    powerOn === undefined
      ? online
        ? "Online"
        : "Offline"
      : !online
        ? "Offline"
        : powerOn
          ? "On"
          : "Off";

  const handleNameSave = useCallback(() => {
    if (nameDraft.trim() && nameDraft.trim() !== name) {
      onUpdateName?.(nameDraft.trim());
    }
    setEditingName(false);
  }, [nameDraft, name, onUpdateName]);

  const handleManual = useCallback(async () => {
    setConfigMode("manual");
    setRegenLoading(true);
    setRegenError(null);
    setConsoleCommands(null);
    try {
      const result = await apiPost(
        `/api/plugs/${plugId}/configure`,
        ConsoleCommandsResultSchema,
        {},
      );
      setConsoleCommands(result.consoleCommands);
    } catch (e) {
      setRegenError(
        e instanceof Error ? e.message : "Failed to regenerate password",
      );
    } finally {
      setRegenLoading(false);
    }
  }, [plugId]);

  const handleAutoConfigSuccess = useCallback(() => {
    setConfigMode(null);
    queryClient.invalidateQueries({ queryKey: queryKeys.plugs.all });
  }, [queryClient]);

  const iconBtn =
    "shrink-0 text-xs text-fg-muted hover:text-accent-muted rounded px-0.5 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-blue-500 transition-colors";

  return (
    <div className="space-y-2">
      {/* Status dot + name + icon buttons */}
      <div className="flex items-center gap-2">
        <span
          role="img"
          className={`h-2 w-2 shrink-0 rounded-full ${dotCls}`}
          aria-label={dotLabel}
        />
        {editingName ? (
          <input
            id={nameInputId}
            type="text"
            value={nameDraft}
            onChange={(e) => setNameDraft(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") handleNameSave();
              if (e.key === "Escape") {
                setNameDraft(name);
                setEditingName(false);
              }
            }}
            ref={focusNameInput}
            className="flex-1 rounded bg-surface-raised border border-border px-2 py-1 text-sm text-fg focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
          />
        ) : (
          <div className="flex items-center gap-1.5 flex-1 min-w-0">
            <span className="text-sm text-fg truncate">{name}</span>
            <button
              type="button"
              onClick={() => {
                setNameDraft(name);
                setEditingName(true);
              }}
              title="Edit name"
              aria-label="Edit name"
              className={iconBtn}
            >
              <i className="fa-solid fa-pen" />
            </button>
            <button
              type="button"
              onClick={() => {
                setDeleteConfirm(false);
                setConfigMode("path");
              }}
              title="Configure"
              aria-label="Configure"
              className={iconBtn}
            >
              <i className="fa-solid fa-gear" />
            </button>
            <button
              type="button"
              onClick={() => {
                setConfigMode(null);
                setDeleteConfirm(true);
              }}
              title="Delete"
              aria-label="Delete"
              className="shrink-0 text-xs text-fg-muted hover:text-danger rounded px-0.5 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-red-500 transition-colors"
            >
              <i className="fa-solid fa-trash-can" />
            </button>
          </div>
        )}
        {/* ON / OFF label for maintenance chargers */}
        {powerOn !== undefined && (
          <span className="text-xs text-fg-muted shrink-0">
            {!online ? "Offline" : powerOn ? "ON" : "OFF"}
          </span>
        )}
      </div>

      {/* Name edit save/cancel */}
      {editingName && (
        <div className="flex gap-2">
          <button
            type="button"
            onClick={() => {
              setNameDraft(name);
              setEditingName(false);
            }}
            className="text-xs text-fg-muted hover:text-fg focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-fg-muted rounded"
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={handleNameSave}
            disabled={!nameDraft.trim()}
            className="text-xs text-accent-muted hover:text-accent disabled:opacity-40 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-blue-500 rounded"
          >
            Save
          </button>
        </div>
      )}

      {/* Configure modal - opens above the settings modal via showModal() */}
      {configMode !== null && (
        <Dialog isOpen onClose={() => setConfigMode(null)}>
          <div className="bg-surface rounded-xl border border-border w-full max-w-md mx-4 p-5">
            <h2 className="text-base font-medium text-fg mb-4">Configure</h2>

            {configMode === "path" && (
              <div className="space-y-3">
                <p className="text-xs text-fg-muted">
                  How do you want to configure this plug?
                </p>
                <div className="grid grid-cols-2 gap-2">
                  <button
                    type="button"
                    onClick={() => setConfigMode("auto")}
                    className="rounded-lg border border-border bg-surface px-3 py-3 text-left hover:border-fg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 transition-colors"
                  >
                    <p className="text-sm font-medium text-fg">
                      Auto-configure
                    </p>
                    <p className="text-xs text-fg-muted mt-0.5">
                      Push settings to device
                    </p>
                  </button>
                  <button
                    type="button"
                    onClick={() => void handleManual()}
                    className="rounded-lg border border-border bg-surface px-3 py-3 text-left hover:border-fg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 transition-colors"
                  >
                    <p className="text-sm font-medium text-fg">Manual</p>
                    <p className="text-xs text-fg-muted mt-0.5">
                      Show console commands
                    </p>
                  </button>
                </div>
                <button
                  type="button"
                  onClick={() => setConfigMode(null)}
                  className="text-xs text-fg-muted hover:text-fg-secondary focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-fg-muted rounded"
                >
                  Cancel
                </button>
              </div>
            )}

            {configMode === "auto" && (
              <AutoConfigureForm
                plugId={plugId}
                onSuccess={handleAutoConfigSuccess}
                onCancel={() => setConfigMode("path")}
              />
            )}

            {configMode === "manual" && (
              <div className="space-y-3">
                {regenLoading && (
                  <p className="text-xs text-fg-muted">
                    <i className="fa-solid fa-spinner fa-spin mr-1" />
                    Generating…
                  </p>
                )}
                {regenError && (
                  <p className="text-xs text-danger">{regenError}</p>
                )}
                {consoleCommands && (
                  <div className="space-y-1">
                    <p className="text-xs text-fg-muted">
                      Paste into Tasmota console. Device will restart.
                    </p>
                    <ConsoleCommandsBlock commands={consoleCommands} />
                  </div>
                )}
                <button
                  type="button"
                  onClick={() => setConfigMode(null)}
                  className="text-xs text-fg-muted hover:text-fg-secondary focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-fg-muted rounded"
                >
                  Done
                </button>
              </div>
            )}
          </div>
        </Dialog>
      )}

      {/* Delete confirmation */}
      {deleteConfirm && (
        <div className="rounded-lg bg-surface-raised border border-border p-3 space-y-2">
          <p className="text-xs text-fg-secondary">{deleteConfirmText}</p>
          <div className="flex gap-2">
            <button
              type="button"
              onClick={() => setDeleteConfirm(false)}
              className="text-xs text-fg-muted hover:text-fg focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-fg-muted rounded"
            >
              Cancel
            </button>
            <button
              type="button"
              onClick={onDelete}
              className="text-xs font-medium text-danger hover:text-danger focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-red-500 rounded"
            >
              Delete
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

// ─── 12V maintenance charger sub-section ─────────────────────────────────────

function Maintenance12VSection({
  plug,
  onDeleteMaintenance,
  onUpdateName,
}: {
  plug: Plug;
  onDeleteMaintenance?: () => void;
  onUpdateName?: (name: string) => void;
}) {
  return (
    <PlugControls
      plugId={plug.id}
      name={plug.name}
      online={plug.online}
      powerOn={plug.powerOn}
      deleteConfirmText="Remove this 12V charger?"
      onUpdateName={onUpdateName}
      onDelete={onDeleteMaintenance}
    />
  );
}
