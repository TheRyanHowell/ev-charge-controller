"use client";

import type { Vehicle, VehicleModel } from "@/lib/schemas";

import ConsoleCommandsBlock from "@/components/ConsoleCommandsBlock";
import ScheduleFormComponent from "@/components/ScheduleForm";
import VehicleSelector from "@/components/VehicleSelector";
import { apiPost, apiGetSingle, apiGet, apiPatch, apiDelete } from "@/lib/api";
import { isPushEnabled, subscribeToPush } from "@/lib/push";
import { queryKeys } from "@/lib/queryKeys";
import {
  PlugSchema,
  ProvisioningResultSchema,
  ConsoleCommandsResultSchema,
  VehicleModelSchema,
  VehicleSchema,
  ScheduleSchema,
} from "@/lib/schemas";
import { parseVehicleSelectorValue } from "@/lib/vehicle-selector";
import { useQuery } from "@tanstack/react-query";
import { useState, useCallback, useEffect, useId, useRef } from "react";

// ─── shared primitives ────────────────────────────────────────────────────────

function SectionTitle({ children }: { children: React.ReactNode }) {
  return <h2 className="text-xl font-semibold text-fg">{children}</h2>;
}

function SubText({ children }: { children: React.ReactNode }) {
  return <p className="text-sm text-fg-muted mt-1">{children}</p>;
}

function PrimaryButton({
  children,
  onClick,
  disabled,
  type = "button",
}: {
  children: React.ReactNode;
  onClick?: () => void;
  disabled?: boolean;
  type?: "button" | "submit";
}) {
  return (
    <button
      type={type}
      onClick={onClick}
      disabled={disabled}
      className="w-full rounded-lg bg-blue-600 px-4 py-2.5 text-sm font-medium text-fg
        hover:bg-blue-500 disabled:opacity-40 disabled:cursor-not-allowed
        focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 transition-colors"
    >
      {children}
    </button>
  );
}

function BackButton({ onClick }: { onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="text-sm text-fg-muted hover:text-fg focus-visible:outline-none
        focus-visible:ring-2 focus-visible:ring-blue-500 rounded transition-colors"
    >
      ← Back
    </button>
  );
}

// ─── step types ───────────────────────────────────────────────────────────────

type Step =
  | "wifi-setup"
  | "path-select"
  | "auto-config"
  | "manual-mqtt"
  | "waiting"
  | "12v-offer"
  | "12v-auto"
  | "12v-manual"
  | "schedule"
  | "notifications"
  | "done";

// ─── step 1: wifi setup ───────────────────────────────────────────────────────

function WifiSetupStep({ onNext }: { onNext: () => void }) {
  return (
    <div className="space-y-6">
      <div>
        <SectionTitle>Connect your plug to Wi-Fi</SectionTitle>
        <SubText>Follow these steps to join your home network.</SubText>
      </div>
      <ol className="space-y-3">
        {[
          "Power on the plug.",
          "On your phone, connect to the Tasmota Wi-Fi hotspot (e.g. tasmota-XXXXXX).",
          'Open a browser and go to 192.168.4.1. Under "Wifi Config", enter your SSID and password.',
          "Save and reconnect your phone to your home Wi-Fi.",
        ].map((step, i) => (
          <li key={i} className="flex gap-3">
            <span
              className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full
              bg-blue-600 text-xs font-bold text-fg"
            >
              {i + 1}
            </span>
            <span className="text-sm text-fg-secondary pt-0.5">{step}</span>
          </li>
        ))}
      </ol>
      <PrimaryButton onClick={onNext}>My plug is on Wi-Fi →</PrimaryButton>
    </div>
  );
}

// ─── step 2: path select ──────────────────────────────────────────────────────

function PathSelectStep({
  onAuto,
  onManual,
  onBack,
}: {
  onAuto: () => void;
  onManual: () => void;
  onBack: () => void;
}) {
  return (
    <div className="space-y-6">
      <div>
        <SectionTitle>How do you want to configure MQTT?</SectionTitle>
        <SubText>We need to push broker settings to the device.</SubText>
      </div>
      <div className="space-y-3">
        <button
          type="button"
          onClick={onAuto}
          className="w-full rounded-lg border border-blue-600/50 bg-blue-600/10 px-4 py-4 text-left
            hover:bg-blue-600/20 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 transition-colors"
        >
          <p className="text-sm font-medium text-fg">Auto-configure</p>
          <p className="text-xs text-fg-muted mt-0.5">
            I know the plug&apos;s IP - push settings automatically
          </p>
        </button>
        <button
          type="button"
          onClick={onManual}
          className="w-full rounded-lg border border-border bg-surface px-4 py-4 text-left
            hover:bg-surface-hover focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 transition-colors"
        >
          <p className="text-sm font-medium text-fg">Manual MQTT</p>
          <p className="text-xs text-fg-muted mt-0.5">
            I&apos;ll copy the credentials into Tasmota myself
          </p>
        </button>
      </div>
      <BackButton onClick={onBack} />
    </div>
  );
}

// ─── step 3a: auto-config ─────────────────────────────────────────────────────

function AutoConfigStep({
  onSuccess,
  onBack,
}: {
  onSuccess: (plugId: string, vehicleId?: string) => void;
  onBack: () => void;
}) {
  const nameId = useId();
  const ipId = useId();
  const passId = useId();
  const vehicleNameId = useId();
  const [name, setName] = useState("");
  const [ip, setIp] = useState("");
  const [tasmotaPass, setTasmotaPass] = useState("");
  const [selectedValue, setSelectedValue] = useState("");
  const [vehicleName, setVehicleName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const { data: models = [] } = useQuery({
    queryKey: queryKeys.vehicleModels.all,
    queryFn: () => apiGet("/api/vehicle-models", VehicleModelSchema),
  });

  const { data: vehicles = [] } = useQuery({
    queryKey: queryKeys.vehicles.all,
    queryFn: () => apiGet("/api/vehicles", VehicleSchema),
  });

  const selection = parseVehicleSelectorValue(selectedValue);
  const isNewModel = selection?.type === "model";

  const handleSubmit = useCallback(async () => {
    if (!name.trim() || !ip.trim() || !selection) return;
    setLoading(true);
    setError(null);
    let vehicleId: string | null = null;
    let plugId: string | null = null;
    try {
      if (selection.type === "vehicle") {
        vehicleId = selection.vehicleId;
      } else {
        const selectedModel = (models as VehicleModel[]).find(
          (m) => m.id === selection.modelId,
        );
        const vName = vehicleName.trim() || selectedModel?.name || "My EV";
        const vehicle = await apiPost("/api/vehicles", VehicleSchema, {
          modelId: selection.modelId,
          name: vName,
        });
        vehicleId = vehicle.id;
      }

      const result = await apiPost("/api/plugs", ProvisioningResultSchema, {
        name,
        vehicleId,
      });
      plugId = result.plug.id;
      await apiPost(
        `/api/plugs/${result.plug.id}/configure`,
        ConsoleCommandsResultSchema,
        {
          tasmotaIP: ip,
          tasmotaPassword: tasmotaPass || undefined,
        },
      );
      onSuccess(result.plug.id, vehicleId ?? undefined);
    } catch (e) {
      // Cleanup on failure
      if (plugId) {
        await apiDelete(`/api/plugs/${plugId}`).catch(() => {});
      }
      if (vehicleId) {
        await apiDelete(`/api/vehicles/${vehicleId}`).catch(() => {});
      }
      setError(e instanceof Error ? e.message : "Something went wrong");
    } finally {
      setLoading(false);
    }
  }, [name, ip, tasmotaPass, selection, vehicleName, models, onSuccess]);

  return (
    <div className="space-y-5">
      <div>
        <SectionTitle>Auto-configure</SectionTitle>
        <SubText>We&apos;ll push MQTT settings directly to the plug.</SubText>
      </div>
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
            placeholder="Driveway"
            className="w-full rounded-lg bg-surface border border-border px-3 py-2 text-sm text-fg
              placeholder-fg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
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
            className="w-full rounded-lg bg-surface border border-border px-3 py-2 text-sm text-fg
              placeholder-fg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
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
            className="w-full rounded-lg bg-surface border border-border px-3 py-2 text-sm text-fg
              placeholder-fg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
          />
        </div>
        <VehicleSelector
          label="Vehicle *"
          vehicles={vehicles as Vehicle[]}
          models={models as VehicleModel[]}
          selectedVehicleId={selectedValue || null}
          onSelectVehicle={(v) => {
            setSelectedValue(v);
            const sel = parseVehicleSelectorValue(v);
            if (sel?.type === "model") {
              const m = (models as VehicleModel[]).find(
                (mod) => mod.id === sel.modelId,
              );
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
              className="w-full rounded-lg bg-surface border border-border px-3 py-2 text-sm text-fg
                placeholder-fg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
            />
          </div>
        )}
      </div>
      {error && <p className="text-xs text-danger">{error}</p>}
      <PrimaryButton
        onClick={() => void handleSubmit()}
        disabled={!name.trim() || !ip.trim() || !selection || loading}
      >
        {loading ? "Configuring…" : "Configure →"}
      </PrimaryButton>
      <BackButton onClick={onBack} />
    </div>
  );
}

// ─── step 3b: manual mqtt ─────────────────────────────────────────────────────

function ManualMqttStep({
  onSuccess,
  onBack,
}: {
  onSuccess: (plugId: string, vehicleId?: string) => void;
  onBack: () => void;
}) {
  const nameId = useId();
  const vehicleNameId = useId();
  const [name, setName] = useState("");
  const [selectedValue, setSelectedValue] = useState("");
  const [vehicleName, setVehicleName] = useState("");
  const [createdPlugId, setCreatedPlugId] = useState<string | null>(null);
  const [createdVehicleId, setCreatedVehicleId] = useState<string | null>(null);
  const [consoleCommands, setConsoleCommands] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const { data: models = [] } = useQuery({
    queryKey: queryKeys.vehicleModels.all,
    queryFn: () => apiGet("/api/vehicle-models", VehicleModelSchema),
  });

  const { data: vehicles = [] } = useQuery({
    queryKey: queryKeys.vehicles.all,
    queryFn: () => apiGet("/api/vehicles", VehicleSchema),
  });

  const selection = parseVehicleSelectorValue(selectedValue);
  const isNewModel = selection?.type === "model";

  const handleCreate = useCallback(async () => {
    if (!name.trim() || !selection) return;
    setLoading(true);
    setError(null);
    let vehicleId: string | null = null;
    let plugId: string | null = null;
    try {
      if (selection.type === "vehicle") {
        vehicleId = selection.vehicleId;
      } else {
        const selectedModel = (models as VehicleModel[]).find(
          (m) => m.id === selection.modelId,
        );
        const vName = vehicleName.trim() || selectedModel?.name || "My EV";
        const vehicle = await apiPost("/api/vehicles", VehicleSchema, {
          modelId: selection.modelId,
          name: vName,
        });
        vehicleId = vehicle.id;
      }

      const r = await apiPost("/api/plugs", ProvisioningResultSchema, {
        name,
        vehicleId,
      });
      plugId = r.plug.id;
      const cfg = await apiPost(
        `/api/plugs/${r.plug.id}/configure`,
        ConsoleCommandsResultSchema,
        {},
      );
      setCreatedPlugId(r.plug.id);
      setCreatedVehicleId(vehicleId);
      setConsoleCommands(cfg.consoleCommands);
    } catch (e) {
      if (plugId) {
        await apiDelete(`/api/plugs/${plugId}`).catch(() => {});
      }
      if (vehicleId) {
        await apiDelete(`/api/vehicles/${vehicleId}`).catch(() => {});
      }
      setError(e instanceof Error ? e.message : "Something went wrong");
    } finally {
      setLoading(false);
    }
  }, [name, selection, vehicleName, models]);

  if (createdPlugId && consoleCommands) {
    return (
      <div className="space-y-5">
        <div>
          <SectionTitle>Paste into Tasmota console</SectionTitle>
          <SubText>
            Open the Tasmota console and paste the command below. Save the
            commands - they won&apos;t be shown again.
          </SubText>
        </div>
        <ConsoleCommandsBlock commands={consoleCommands} />
        <PrimaryButton
          onClick={() =>
            onSuccess(createdPlugId, createdVehicleId ?? undefined)
          }
        >
          I&apos;ve pasted the command →
        </PrimaryButton>
        <BackButton onClick={onBack} />
      </div>
    );
  }

  return (
    <div className="space-y-5">
      <div>
        <SectionTitle>Manual MQTT setup</SectionTitle>
        <SubText>
          Give your plug a name and select a vehicle to generate credentials.
        </SubText>
      </div>
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
            placeholder="Driveway"
            className="w-full rounded-lg bg-surface border border-border px-3 py-2 text-sm text-fg
              placeholder-fg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
          />
        </div>
        <VehicleSelector
          label="Vehicle *"
          vehicles={vehicles as Vehicle[]}
          models={models as VehicleModel[]}
          selectedVehicleId={selectedValue || null}
          onSelectVehicle={(v) => {
            setSelectedValue(v);
            const sel = parseVehicleSelectorValue(v);
            if (sel?.type === "model") {
              const m = (models as VehicleModel[]).find(
                (mod) => mod.id === sel.modelId,
              );
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
              className="w-full rounded-lg bg-surface border border-border px-3 py-2 text-sm text-fg
                placeholder-fg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
            />
          </div>
        )}
      </div>
      {error && <p className="text-xs text-danger">{error}</p>}
      <PrimaryButton
        onClick={() => void handleCreate()}
        disabled={!name.trim() || !selection || loading}
      >
        {loading ? "Creating…" : "Generate credentials →"}
      </PrimaryButton>
      <BackButton onClick={onBack} />
    </div>
  );
}

// ─── step 4: waiting ──────────────────────────────────────────────────────────

const POLL_INTERVAL_MS = 3000;
const TIMEOUT_MS = 120_000;

function WaitingStep({
  plugId,
  onSuccess,
  onBack,
}: {
  plugId: string;
  onSuccess: () => void;
  onBack: () => void;
}) {
  const startRef = useRef(0);
  const [timedOut, setTimedOut] = useState(false);
  const [seenOnline, setSeenOnline] = useState(false);

  useEffect(() => {
    startRef.current = Date.now();
  }, []);

  const { data: plug } = useQuery({
    queryKey: queryKeys.plugs.byId(plugId),
    queryFn: () => apiGetSingle(`/api/plugs/${plugId}`, PlugSchema),
    refetchInterval: timedOut ? false : POLL_INTERVAL_MS,
    enabled: !timedOut,
  });

  useEffect(() => {
    if (!plug) return;
    if (plug.online && !seenOnline) {
      setSeenOnline(true);
      onSuccess();
    }
    if (startRef.current && Date.now() - startRef.current > TIMEOUT_MS) {
      setTimedOut(true);
    }
  }, [plug, seenOnline, onSuccess]);

  if (timedOut) {
    return (
      <div className="space-y-5">
        <SectionTitle>Taking longer than expected</SectionTitle>
        <SubText>
          Check that the plug&apos;s MQTT settings are correct and it has
          internet access.
        </SubText>
        <PrimaryButton
          onClick={() => {
            setTimedOut(false);
            startRef.current = Date.now();
          }}
        >
          Try again
        </PrimaryButton>
        <BackButton onClick={onBack} />
      </div>
    );
  }

  return (
    <div className="space-y-6 text-center">
      <div className="mx-auto h-12 w-12 rounded-full border-4 border-blue-600 border-t-transparent animate-spin" />
      <div>
        <SectionTitle>
          {seenOnline ? "Configuring device…" : "Waiting for plug to connect…"}
        </SectionTitle>
        <SubText>This can take up to 2 minutes after restarting.</SubText>
      </div>
      <BackButton onClick={onBack} />
    </div>
  );
}

// ─── step 6: schedule ─────────────────────────────────────────────────────────

function ScheduleStep({
  plugId,
  onNext,
  onBack,
}: {
  plugId: string;
  onNext: () => void;
  onBack: () => void;
}) {
  const [saving, setSaving] = useState(false);

  const handleSave = useCallback(
    async (payload: import("@/hooks/useSchedule").SchedulePayload) => {
      setSaving(true);
      try {
        await apiPatch(
          `/api/plugs/${plugId}/schedule`,
          ScheduleSchema,
          payload as unknown as Record<string, unknown>,
        );
      } catch {
        // Non-fatal in wizard - user can configure later from Settings.
      } finally {
        setSaving(false);
        onNext();
      }
    },
    [plugId, onNext],
  );

  return (
    <div className="space-y-5">
      <div>
        <SectionTitle>Set a charging schedule</SectionTitle>
        <SubText>
          Optional - auto-start charging at a set time. Supports daily and
          carbon-aware modes.
        </SubText>
      </div>
      <ScheduleFormComponent
        schedule={null}
        onSave={handleSave}
        isSaving={saving}
        onSkip={onNext}
        saveLabel="Save &amp; continue →"
      />
      <BackButton onClick={onBack} />
    </div>
  );
}

// ─── step 6: notifications ────────────────────────────────────────────────────

function NotificationsStep({
  onNext,
  onBack,
}: {
  onNext: () => void;
  onBack: () => void;
}) {
  const [subscribed, setSubscribed] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubscribe = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const sub = await subscribeToPush();
      setSubscribed(!!sub);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to subscribe");
    } finally {
      setLoading(false);
    }
  }, []);

  const handleNext = useCallback(() => {
    onNext();
  }, [onNext]);

  if (!isPushEnabled()) {
    return (
      <div className="space-y-5">
        <div>
          <SectionTitle>Notifications</SectionTitle>
          <SubText>
            Push notifications aren&apos;t available in this browser. You can
            still use the app without them.
          </SubText>
        </div>
        <PrimaryButton onClick={handleNext}>Continue →</PrimaryButton>
        <BackButton onClick={onBack} />
      </div>
    );
  }

  return (
    <div className="space-y-5">
      <div>
        <SectionTitle>Enable notifications</SectionTitle>
        <SubText>
          Get alerted when charging completes or your plug goes offline.
        </SubText>
      </div>
      <div className="rounded-lg bg-surface border border-border px-4 py-3">
        {subscribed ? (
          <div className="flex items-center gap-2">
            <svg
              className="h-5 w-5 text-success shrink-0"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={2}
              aria-hidden="true"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M5 13l4 4L19 7"
              />
            </svg>
            <p className="text-sm text-fg">Notifications enabled</p>
          </div>
        ) : (
          <div className="space-y-2">
            <p className="text-sm text-fg-secondary">
              We&apos;ll send you a browser notification when charging finishes.
            </p>
            <button
              type="button"
              onClick={() => void handleSubscribe()}
              disabled={loading}
              className="w-full rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-fg
                hover:bg-blue-500 disabled:opacity-40 disabled:cursor-not-allowed
                focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 transition-colors"
            >
              {loading ? "Subscribing…" : "Enable notifications"}
            </button>
          </div>
        )}
      </div>
      {error && <p className="text-xs text-danger">{error}</p>}
      <PrimaryButton onClick={handleNext}>Continue →</PrimaryButton>
      <BackButton onClick={onBack} />
    </div>
  );
}

// ─── step 5a: offer 12V charger ──────────────────────────────────────────────

function Offer12VStep({
  onAdd,
  onAddManual,
  onSkip,
}: {
  onAdd: () => void;
  onAddManual: () => void;
  onSkip: () => void;
}) {
  return (
    <div className="space-y-6">
      <div>
        <SectionTitle>Add a 12V maintenance charger?</SectionTitle>
        <SubText>
          Keep your motorcycle&apos;s 12V battery topped up with an always-on
          Tasmota smart plug.
        </SubText>
      </div>
      <div className="grid grid-cols-2 gap-2">
        <button
          type="button"
          onClick={onAdd}
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
          onClick={onAddManual}
          className="rounded-lg border border-border bg-surface px-3 py-3 text-left hover:border-fg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 transition-colors"
        >
          <p className="text-sm font-medium text-fg">Manual MQTT</p>
          <p className="text-xs text-fg-muted mt-0.5">
            Enter settings in Tasmota
          </p>
        </button>
      </div>
      <button
        type="button"
        onClick={onSkip}
        className="text-sm text-fg-muted hover:text-fg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 rounded transition-colors"
      >
        Skip →
      </button>
    </div>
  );
}

// ─── step 5b: auto-config 12V charger ────────────────────────────────────────

function AutoConfig12VStep({
  vehicleId,
  onSuccess,
  onBack,
}: {
  vehicleId: string;
  onSuccess: () => void;
  onBack: () => void;
}) {
  const nameId = useId();
  const ipId = useId();
  const passId = useId();
  const [name, setName] = useState("12V Charger");
  const [ip, setIp] = useState("");
  const [tasmotaPass, setTasmotaPass] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const handleSubmit = useCallback(async () => {
    if (!name.trim() || !ip.trim()) return;
    setLoading(true);
    setError(null);
    let plugId: string | null = null;
    try {
      const result = await apiPost("/api/plugs", ProvisioningResultSchema, {
        name,
        vehicleId,
        type: "maintenance",
      });
      plugId = result.plug.id;
      await apiPost(
        `/api/plugs/${result.plug.id}/configure`,
        ConsoleCommandsResultSchema,
        {
          tasmotaIP: ip,
          tasmotaPassword: tasmotaPass || undefined,
        },
      );
      onSuccess();
    } catch (e) {
      if (plugId) await apiDelete(`/api/plugs/${plugId}`).catch(() => {});
      setError(e instanceof Error ? e.message : "Something went wrong");
    } finally {
      setLoading(false);
    }
  }, [name, ip, tasmotaPass, vehicleId, onSuccess]);

  return (
    <div className="space-y-5">
      <div>
        <SectionTitle>Auto-configure 12V charger</SectionTitle>
        <SubText>
          We&apos;ll push MQTT settings to the maintenance charger.
        </SubText>
      </div>
      <div className="space-y-3">
        <div>
          <label htmlFor={nameId} className="block text-xs text-fg-muted mb-1">
            Charger name *
          </label>
          <input
            id={nameId}
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="12V Charger"
            className="w-full rounded-lg bg-surface border border-border px-3 py-2 text-sm text-fg placeholder-fg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
          />
        </div>
        <div>
          <label htmlFor={ipId} className="block text-xs text-fg-muted mb-1">
            Device IP address *
          </label>
          <input
            id={ipId}
            type="text"
            value={ip}
            onChange={(e) => setIp(e.target.value)}
            placeholder="192.168.1.51"
            className="w-full rounded-lg bg-surface border border-border px-3 py-2 text-sm text-fg placeholder-fg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
          />
        </div>
        <div>
          <label htmlFor={passId} className="block text-xs text-fg-muted mb-1">
            Tasmota password (optional)
          </label>
          <input
            id={passId}
            type="password"
            value={tasmotaPass}
            onChange={(e) => setTasmotaPass(e.target.value)}
            placeholder="Leave blank if none"
            className="w-full rounded-lg bg-surface border border-border px-3 py-2 text-sm text-fg placeholder-fg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
          />
        </div>
      </div>
      {error && <p className="text-xs text-danger">{error}</p>}
      <PrimaryButton
        onClick={() => void handleSubmit()}
        disabled={!name.trim() || !ip.trim() || loading}
      >
        {loading ? "Configuring…" : "Configure →"}
      </PrimaryButton>
      <BackButton onClick={onBack} />
    </div>
  );
}

// ─── step 5c: manual 12V charger ─────────────────────────────────────────────

function Manual12VStep({
  vehicleId,
  onSuccess,
  onBack,
}: {
  vehicleId: string;
  onSuccess: () => void;
  onBack: () => void;
}) {
  const nameId = useId();
  const ipId = useId();
  const [name, setName] = useState("12V Charger");
  const [ip, setIp] = useState("");
  const [consoleCommands, setConsoleCommands] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const handleCreate = useCallback(async () => {
    if (!name.trim()) return;
    setLoading(true);
    setError(null);
    let plugId: string | null = null;
    try {
      const r = await apiPost("/api/plugs", ProvisioningResultSchema, {
        name,
        vehicleId,
        type: "maintenance",
      });
      plugId = r.plug.id;
      const cfg = await apiPost(
        `/api/plugs/${r.plug.id}/configure`,
        ConsoleCommandsResultSchema,
        {},
      );
      setConsoleCommands(cfg.consoleCommands);
    } catch (e) {
      if (plugId) await apiDelete(`/api/plugs/${plugId}`).catch(() => {});
      setError(e instanceof Error ? e.message : "Something went wrong");
    } finally {
      setLoading(false);
    }
  }, [name, vehicleId]);

  if (consoleCommands) {
    return (
      <div className="space-y-5">
        <div>
          <SectionTitle>Paste into Tasmota console</SectionTitle>
          <SubText>Open the Tasmota console and paste all lines below.</SubText>
        </div>
        <ConsoleCommandsBlock commands={consoleCommands} />
        <PrimaryButton onClick={onSuccess}>
          I&apos;ve pasted the command →
        </PrimaryButton>
        <BackButton onClick={onBack} />
      </div>
    );
  }

  return (
    <div className="space-y-5">
      <div>
        <SectionTitle>Manual 12V charger setup</SectionTitle>
        <SubText>
          Generate MQTT credentials for the maintenance charger.
        </SubText>
      </div>
      <div className="space-y-3">
        <div>
          <label htmlFor={nameId} className="block text-xs text-fg-muted mb-1">
            Charger name *
          </label>
          <input
            id={nameId}
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="12V Charger"
            className="w-full rounded-lg bg-surface border border-border px-3 py-2 text-sm text-fg placeholder-fg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
          />
        </div>
        <div>
          <label htmlFor={ipId} className="block text-xs text-fg-muted mb-1">
            Device IP (optional, for display)
          </label>
          <input
            id={ipId}
            type="text"
            value={ip}
            onChange={(e) => setIp(e.target.value)}
            placeholder="192.168.1.51"
            className="w-full rounded-lg bg-surface border border-border px-3 py-2 text-sm text-fg placeholder-fg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
          />
        </div>
      </div>
      {error && <p className="text-xs text-danger">{error}</p>}
      <PrimaryButton
        onClick={() => void handleCreate()}
        disabled={!name.trim() || loading}
      >
        {loading ? "Creating…" : "Generate credentials →"}
      </PrimaryButton>
      <BackButton onClick={onBack} />
    </div>
  );
}

// ─── step 7: done ─────────────────────────────────────────────────────────────

function DoneStep({ onComplete }: { onComplete: () => void }) {
  return (
    <div className="space-y-6 text-center">
      <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-full bg-green-500/20">
        <svg
          className="h-7 w-7 text-success"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          strokeWidth={2}
          aria-hidden="true"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            d="M5 13l4 4L19 7"
          />
        </svg>
      </div>
      <div>
        <SectionTitle>You&apos;re set up!</SectionTitle>
        <SubText>The plug is online and ready to charge.</SubText>
      </div>
      <PrimaryButton onClick={onComplete}>Go to dashboard →</PrimaryButton>
    </div>
  );
}

// ─── main wizard ─────────────────────────────────────────────────────────────

interface FirstRunWizardProps {
  onComplete: () => Promise<void> | void;
}

export default function FirstRunWizard({ onComplete }: FirstRunWizardProps) {
  const [step, setStep] = useState<Step>("wifi-setup");
  const [plugId, setPlugId] = useState<string | null>(null);
  // Vehicle ID of the charging plug - used for 12V step provisioning.
  const [vehicleId, setVehicleId] = useState<string | null>(null);

  const handleStepChange = useCallback((newStep: Step) => {
    setStep(newStep);
  }, []);

  const handleComplete = useCallback(() => {
    void onComplete();
  }, [onComplete]);

  return (
    <main className="min-h-screen bg-page-bg text-fg flex items-center justify-center p-4">
      <div className="w-full max-w-md">
        <div className="mb-8">
          <h1 className="text-2xl font-bold text-fg">EV Charge Controller</h1>
          <p className="text-fg-muted text-sm mt-1">Setup</p>
        </div>
        <div className="bg-surface-raised rounded-xl border border-border p-6">
          {step === "wifi-setup" && (
            <WifiSetupStep onNext={() => handleStepChange("path-select")} />
          )}
          {step === "path-select" && (
            <PathSelectStep
              onAuto={() => handleStepChange("auto-config")}
              onManual={() => handleStepChange("manual-mqtt")}
              onBack={() => handleStepChange("wifi-setup")}
            />
          )}
          {step === "auto-config" && (
            <AutoConfigStep
              onSuccess={(id, vid) => {
                setPlugId(id);
                setVehicleId(vid ?? null);
                handleStepChange("waiting");
              }}
              onBack={() => handleStepChange("path-select")}
            />
          )}
          {step === "manual-mqtt" && (
            <ManualMqttStep
              onSuccess={(id, vid) => {
                setPlugId(id);
                setVehicleId(vid ?? null);
                handleStepChange("waiting");
              }}
              onBack={() => handleStepChange("path-select")}
            />
          )}
          {step === "waiting" && plugId && (
            <WaitingStep
              plugId={plugId}
              onSuccess={() => handleStepChange("12v-offer")}
              onBack={() => handleStepChange("path-select")}
            />
          )}
          {step === "12v-offer" && (
            <Offer12VStep
              onAdd={() => handleStepChange("12v-auto")}
              onAddManual={() => handleStepChange("12v-manual")}
              onSkip={() => handleStepChange("schedule")}
            />
          )}
          {step === "12v-auto" && vehicleId && (
            <AutoConfig12VStep
              vehicleId={vehicleId}
              onSuccess={() => handleStepChange("schedule")}
              onBack={() => handleStepChange("12v-offer")}
            />
          )}
          {step === "12v-manual" && vehicleId && (
            <Manual12VStep
              vehicleId={vehicleId}
              onSuccess={() => handleStepChange("schedule")}
              onBack={() => handleStepChange("12v-offer")}
            />
          )}
          {step === "schedule" && plugId && (
            <ScheduleStep
              plugId={plugId}
              onNext={() => handleStepChange("notifications")}
              onBack={() => handleStepChange("12v-offer")}
            />
          )}
          {step === "notifications" && plugId && (
            <NotificationsStep
              onNext={() => handleStepChange("done")}
              onBack={() => handleStepChange("schedule")}
            />
          )}
          {step === "done" && plugId && (
            <DoneStep onComplete={() => void handleComplete()} />
          )}
        </div>
      </div>
    </main>
  );
}
