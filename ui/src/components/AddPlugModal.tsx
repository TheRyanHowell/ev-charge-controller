"use client";

import type {
  Plug,
  Vehicle,
  VehicleModel,
  ProvisioningResult,
} from "@/lib/schemas";

import ConsoleCommandsBlock from "@/components/ConsoleCommandsBlock";
import Dialog from "@/components/Dialog";
import VehicleSelector from "@/components/VehicleSelector";
import { apiPost, apiGet } from "@/lib/api";
import { queryKeys } from "@/lib/queryKeys";
import {
  ProvisioningResultSchema,
  ConsoleCommandsResultSchema,
  VehicleModelSchema,
  VehicleSchema,
} from "@/lib/schemas";
import { parseVehicleSelectorValue } from "@/lib/vehicle-selector";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState, useCallback, useId } from "react";

interface AddPlugModalProps {
  isOpen: boolean;
  onClose: () => void;
  onPlugCreated: (result: ProvisioningResult) => void;
  /** When set, skip vehicle creation and create a maintenance plug for this vehicle. */
  existingVehicleId?: string | null;
}

export default function AddPlugModal({
  isOpen,
  onClose,
  onPlugCreated,
  existingVehicleId,
}: AddPlugModalProps) {
  const is12VMode = !!existingVehicleId;
  const [mode, setMode] = useState<
    | "path-select"
    | "auto-config"
    | "manual"
    | "12v-offer"
    | "12v-auto"
    | "12v-manual"
  >("path-select");
  // Track the vehicleId from the charging plug creation, used in the 12V offer step.
  const [createdVehicleId, setCreatedVehicleId] = useState<string | null>(null);

  // Parent-level queries: fetched once, shared by both form variants.
  // This eliminates duplicate useQuery hooks in child components and
  // ensures data is available before the forms mount.
  const { data: models = [] } = useQuery({
    queryKey: queryKeys.vehicleModels.all,
    queryFn: () => apiGet("/api/vehicle-models", VehicleModelSchema),
    enabled: isOpen,
  });

  const { data: vehicles = [] } = useQuery({
    queryKey: queryKeys.vehicles.all,
    queryFn: () => apiGet("/api/vehicles", VehicleSchema),
    enabled: isOpen,
  });

  // When in 12V mode, treat the modal as if the user picked "add 12V charger".
  const effectiveMode = is12VMode
    ? mode === "path-select"
      ? "12v-offer"
      : mode
    : mode;

  const handleClose = useCallback(() => {
    setMode("path-select");
    setCreatedVehicleId(null);
    onClose();
  }, [onClose]);

  const handleBack = useCallback(() => {
    setMode("path-select");
  }, []);

  // After creating the charging plug, offer the 12V step.
  const handleChargingPlugCreated = useCallback(
    (result: ProvisioningResult) => {
      const vid = result.plug.vehicleId ?? null;
      setCreatedVehicleId(vid);
      // Notify parent now so the vehicle+plug appear in the selector.
      onPlugCreated(result);
      setMode("12v-offer");
    },
    [onPlugCreated],
  );

  const handle12VPlugCreated = useCallback(
    (result: ProvisioningResult) => {
      onPlugCreated(result);
      handleClose();
    },
    [onPlugCreated, handleClose],
  );

  const title = is12VMode
    ? "Add 12V maintenance charger"
    : effectiveMode === "12v-offer"
      ? "Add 12V maintenance charger?"
      : "Add vehicle";

  const vehicleIdFor12V = existingVehicleId ?? createdVehicleId;

  return (
    <Dialog isOpen={isOpen} onClose={handleClose}>
      <div className="bg-surface rounded-xl border border-border w-full max-w-md mx-4 p-5">
        <h2 className="text-base font-medium text-fg mb-4">{title}</h2>

        {effectiveMode === "path-select" && (
          <div className="space-y-3">
            <p className="text-xs text-fg-muted">
              How do you want to set up this plug?
            </p>
            <div className="grid grid-cols-2 gap-2">
              <button
                type="button"
                onClick={() => setMode("auto-config")}
                className="rounded-lg border border-border bg-surface px-3 py-3 text-left hover:border-fg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 transition-colors"
              >
                <p className="text-sm font-medium text-fg">Auto-configure</p>
                <p className="text-xs text-fg-muted mt-0.5">
                  Push MQTT settings to the plug
                </p>
              </button>
              <button
                type="button"
                onClick={() => setMode("manual")}
                className="rounded-lg border border-border bg-surface px-3 py-3 text-left hover:border-fg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 transition-colors"
              >
                <p className="text-sm font-medium text-fg">Manual</p>
                <p className="text-xs text-fg-muted mt-0.5">
                  Enter MQTT settings in Tasmota
                </p>
              </button>
            </div>
            <button
              type="button"
              onClick={handleClose}
              className="text-xs text-fg-muted hover:text-fg-secondary focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-blue-500 rounded"
            >
              Cancel
            </button>
          </div>
        )}

        {effectiveMode === "auto-config" && (
          <AutoConfigAddPlugForm
            models={models as VehicleModel[]}
            vehicles={vehicles as Vehicle[]}
            onSuccess={handleChargingPlugCreated}
            onBack={handleBack}
          />
        )}

        {effectiveMode === "manual" && (
          <ManualAddPlugForm
            models={models as VehicleModel[]}
            vehicles={vehicles as Vehicle[]}
            onSuccess={handleChargingPlugCreated}
            onBack={handleBack}
          />
        )}

        {effectiveMode === "12v-offer" && (
          <div className="space-y-4">
            <p className="text-sm text-fg-secondary">
              Would you like to add a 12V battery maintenance charger for this
              vehicle? It stays on continuously to keep the 12V battery topped
              up.
            </p>
            <div className="grid grid-cols-2 gap-2">
              <button
                type="button"
                onClick={() => setMode("12v-auto")}
                className="rounded-lg border border-cyan-300 bg-cyan-50 dark:border-cyan-700 dark:bg-cyan-900/30 px-3 py-3 text-left hover:border-cyan-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-500 transition-colors"
              >
                <p className="text-sm font-medium text-cyan-700 dark:text-cyan-300">
                  Auto-configure
                </p>
                <p className="text-xs text-fg-muted mt-0.5">
                  Push settings to device
                </p>
              </button>
              <button
                type="button"
                onClick={() => setMode("12v-manual")}
                className="rounded-lg border border-border bg-surface px-3 py-3 text-left hover:border-fg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 transition-colors"
              >
                <p className="text-sm font-medium text-fg">Manual</p>
                <p className="text-xs text-fg-muted mt-0.5">
                  Enter settings in Tasmota
                </p>
              </button>
            </div>
            <button
              type="button"
              onClick={handleClose}
              className="text-xs text-fg-muted hover:text-fg-secondary focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-fg-muted rounded"
            >
              {is12VMode ? "Cancel" : "Skip"}
            </button>
          </div>
        )}

        {effectiveMode === "12v-auto" && vehicleIdFor12V && (
          <AutoConfigAddPlugForm
            models={[]}
            vehicles={[]}
            vehicleId={vehicleIdFor12V}
            plugType="maintenance"
            onSuccess={handle12VPlugCreated}
            onBack={() => setMode("12v-offer")}
          />
        )}

        {effectiveMode === "12v-manual" && vehicleIdFor12V && (
          <ManualAddPlugForm
            models={[]}
            vehicles={[]}
            vehicleId={vehicleIdFor12V}
            plugType="maintenance"
            onSuccess={handle12VPlugCreated}
            onBack={() => setMode("12v-offer")}
          />
        )}
      </div>
    </Dialog>
  );
}

interface AddPlugFormProps {
  models: VehicleModel[];
  vehicles: Vehicle[];
  onSuccess: (result: ProvisioningResult) => void;
  onBack: () => void;
  /** When set, skip vehicle selection and use this vehicle ID directly. */
  vehicleId?: string;
  /** Plug type to create. Defaults to "charging". */
  plugType?: "charging" | "maintenance";
}

function AutoConfigAddPlugForm({
  models,
  vehicles,
  onSuccess,
  onBack,
  vehicleId: fixedVehicleId,
  plugType = "charging",
}: AddPlugFormProps) {
  const queryClient = useQueryClient();
  const nameId = useId();
  const ipId = useId();
  const passId = useId();
  const vehicleNameId = useId();
  const defaultName = plugType === "maintenance" ? "12V Charger" : "";
  const [name, setName] = useState(defaultName);
  const [ip, setIp] = useState("");
  const [tasmotaPass, setTasmotaPass] = useState("");
  const [selectedValue, setSelectedValue] = useState("");
  const [vehicleName, setVehicleName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const selection = parseVehicleSelectorValue(selectedValue);
  const isNewModel = selection?.type === "model";
  // In 12V mode with a fixed vehicleId, skip vehicle selection entirely.
  const needsVehicleSelection = !fixedVehicleId;

  const handleSubmit = useCallback(async () => {
    if (!name.trim() || !ip.trim() || (!fixedVehicleId && !selection)) return;
    setLoading(true);
    setError(null);
    try {
      let resolvedVehicleId: string;

      if (fixedVehicleId) {
        resolvedVehicleId = fixedVehicleId;
      } else if (!selection) {
        return;
      } else if (selection.type === "vehicle") {
        resolvedVehicleId = selection.vehicleId;
      } else {
        const selectedModel = models.find((m) => m.id === selection.modelId);
        const vName = vehicleName.trim() || selectedModel?.name || "My EV";
        const vehicle = await apiPost("/api/vehicles", VehicleSchema, {
          modelId: selection.modelId,
          name: vName,
        });
        queryClient.setQueryData(
          queryKeys.vehicles.all,
          (old: Vehicle[] = []) =>
            old.some((v) => v.id === vehicle.id) ? old : [...old, vehicle],
        );
        resolvedVehicleId = vehicle.id;
      }

      const result = await apiPost("/api/plugs", ProvisioningResultSchema, {
        name,
        vehicleId: resolvedVehicleId,
        type: plugType,
      });
      await apiPost(
        `/api/plugs/${result.plug.id}/configure`,
        ConsoleCommandsResultSchema,
        {
          tasmotaIP: ip,
          tasmotaPassword: tasmotaPass || undefined,
        },
      );
      onSuccess(result);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Something went wrong");
    } finally {
      setLoading(false);
    }
  }, [
    name,
    ip,
    tasmotaPass,
    selection,
    vehicleName,
    models,
    onSuccess,
    queryClient,
    fixedVehicleId,
    plugType,
  ]);

  return (
    <div className="space-y-3">
      <p className="text-xs text-fg-muted">
        {plugType === "maintenance"
          ? "We'll push MQTT settings to the 12V maintenance charger."
          : "We'll push MQTT settings directly to the plug."}
      </p>
      <div>
        <label htmlFor={nameId} className="block text-xs text-fg-muted mb-1">
          Plug name *
        </label>
        <input
          id={nameId}
          type="text"
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder={plugType === "maintenance" ? "12V Charger" : "Driveway"}
          className="w-full rounded bg-surface-raised border border-border px-2.5 py-1.5 text-sm text-fg placeholder-fg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
        />
      </div>
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
      {needsVehicleSelection && (
        <>
          <VehicleSelector
            label="Vehicle *"
            vehicles={vehicles}
            models={models}
            selectedVehicleId={selectedValue || null}
            onSelectVehicle={(v) => {
              setSelectedValue(v);
              const sel = parseVehicleSelectorValue(v);
              if (sel?.type === "model") {
                const m = models.find((mod) => mod.id === sel.modelId);
                if (m) setVehicleName(m.name);
              }
            }}
          />
          {isNewModel && (
            <div>
              <label
                htmlFor={vehicleNameId}
                className="block text-xs text-fg-muted mb-1"
              >
                Nickname (optional)
              </label>
              <input
                id={vehicleNameId}
                type="text"
                value={vehicleName}
                onChange={(e) => setVehicleName(e.target.value)}
                placeholder="My EV"
                className="w-full rounded bg-surface-raised border border-border px-2.5 py-1.5 text-sm text-fg placeholder-fg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
              />
            </div>
          )}
        </>
      )}
      {error && <p className="text-xs text-danger">{error}</p>}
      <div className="flex gap-2">
        <button
          type="button"
          onClick={onBack}
          disabled={loading}
          className="flex-1 rounded bg-surface px-3 py-1.5 text-xs text-fg-secondary hover:bg-surface-hover disabled:opacity-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 transition-colors"
        >
          Back
        </button>
        <button
          type="button"
          onClick={() => void handleSubmit()}
          disabled={
            !name.trim() ||
            !ip.trim() ||
            (needsVehicleSelection && !selection) ||
            loading
          }
          className="flex-1 rounded bg-blue-600 px-3 py-1.5 text-xs font-medium text-fg hover:bg-blue-500 disabled:opacity-40 disabled:cursor-not-allowed focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 transition-colors"
        >
          {loading ? "Configuring…" : "Configure →"}
        </button>
      </div>
    </div>
  );
}

function ManualAddPlugForm({
  models,
  vehicles,
  onSuccess,
  onBack,
  vehicleId: fixedVehicleId,
  plugType = "charging",
}: AddPlugFormProps) {
  const queryClient = useQueryClient();
  const nameId = useId();
  const vehicleNameId = useId();
  const defaultName = plugType === "maintenance" ? "12V Charger" : "";
  const [name, setName] = useState(defaultName);
  const [selectedValue, setSelectedValue] = useState("");
  const [vehicleName, setVehicleName] = useState("");
  const [plug, setPlug] = useState<Plug | null>(null);
  const [consoleCommands, setConsoleCommands] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const selection = parseVehicleSelectorValue(selectedValue);
  const isNewModel = selection?.type === "model";
  const needsVehicleSelection = !fixedVehicleId;

  const handleCreate = useCallback(async () => {
    if (!name.trim() || (!fixedVehicleId && !selection)) return;
    setLoading(true);
    setError(null);
    try {
      let resolvedVehicleId: string;

      if (fixedVehicleId) {
        resolvedVehicleId = fixedVehicleId;
      } else if (!selection) {
        return;
      } else if (selection.type === "vehicle") {
        resolvedVehicleId = selection.vehicleId;
      } else {
        const selectedModel = models.find((m) => m.id === selection.modelId);
        const vName = vehicleName.trim() || selectedModel?.name || "My EV";
        const vehicle = await apiPost("/api/vehicles", VehicleSchema, {
          modelId: selection.modelId,
          name: vName,
        });
        queryClient.setQueryData(
          queryKeys.vehicles.all,
          (old: Vehicle[] = []) =>
            old.some((v) => v.id === vehicle.id) ? old : [...old, vehicle],
        );
        resolvedVehicleId = vehicle.id;
      }

      const r = await apiPost("/api/plugs", ProvisioningResultSchema, {
        name,
        vehicleId: resolvedVehicleId,
        type: plugType,
      });
      const cfg = await apiPost(
        `/api/plugs/${r.plug.id}/configure`,
        ConsoleCommandsResultSchema,
        {},
      );
      setPlug(r.plug);
      setConsoleCommands(cfg.consoleCommands);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Something went wrong");
    } finally {
      setLoading(false);
    }
  }, [
    name,
    selection,
    vehicleName,
    models,
    queryClient,
    fixedVehicleId,
    plugType,
  ]);

  const handleDone = useCallback(() => {
    if (plug) {
      onSuccess({ plug });
    }
  }, [plug, onSuccess]);

  if (plug && consoleCommands) {
    return (
      <div className="space-y-3">
        <p className="text-xs text-fg-muted">
          Open the Tasmota console (Console tab) and paste all lines below. Save
          the commands - they won&apos;t be shown again.
        </p>
        <ConsoleCommandsBlock commands={consoleCommands} />
        <div className="flex gap-2">
          <button
            type="button"
            onClick={onBack}
            className="flex-1 rounded bg-surface px-3 py-1.5 text-xs text-fg-secondary hover:bg-surface-hover focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 transition-colors"
          >
            Back
          </button>
          <button
            type="button"
            onClick={handleDone}
            className="flex-1 rounded bg-blue-600 px-3 py-1.5 text-xs font-medium text-fg hover:bg-blue-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 transition-colors"
          >
            Done ✓
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-3">
      <div>
        <label htmlFor={nameId} className="block text-xs text-fg-muted mb-1">
          Plug name *
        </label>
        <input
          id={nameId}
          type="text"
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder={plugType === "maintenance" ? "12V Charger" : "Driveway"}
          className="w-full rounded bg-surface-raised border border-border px-2.5 py-1.5 text-sm text-fg placeholder-fg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
        />
      </div>
      {needsVehicleSelection && (
        <>
          <VehicleSelector
            label="Vehicle *"
            vehicles={vehicles}
            models={models}
            selectedVehicleId={selectedValue || null}
            onSelectVehicle={(v) => {
              setSelectedValue(v);
              const sel = parseVehicleSelectorValue(v);
              if (sel?.type === "model") {
                const m = models.find((mod) => mod.id === sel.modelId);
                if (m) setVehicleName(m.name);
              }
            }}
          />
          {isNewModel && (
            <div>
              <label
                htmlFor={vehicleNameId}
                className="block text-xs text-fg-muted mb-1"
              >
                Nickname (optional)
              </label>
              <input
                id={vehicleNameId}
                type="text"
                value={vehicleName}
                onChange={(e) => setVehicleName(e.target.value)}
                placeholder="My EV"
                className="w-full rounded bg-surface-raised border border-border px-2.5 py-1.5 text-sm text-fg placeholder-fg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
              />
            </div>
          )}
        </>
      )}
      {error && <p className="text-xs text-danger">{error}</p>}
      <div className="flex gap-2">
        <button
          type="button"
          onClick={onBack}
          disabled={loading}
          className="flex-1 rounded bg-surface px-3 py-1.5 text-xs text-fg-secondary hover:bg-surface-hover disabled:opacity-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 transition-colors"
        >
          Back
        </button>
        <button
          type="button"
          onClick={() => void handleCreate()}
          disabled={
            !name.trim() || (needsVehicleSelection && !selection) || loading
          }
          className="flex-1 rounded bg-blue-600 px-3 py-1.5 text-xs font-medium text-fg hover:bg-blue-500 disabled:opacity-40 disabled:cursor-not-allowed focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 transition-colors"
        >
          {loading ? "Creating…" : "Generate credentials →"}
        </button>
      </div>
    </div>
  );
}
