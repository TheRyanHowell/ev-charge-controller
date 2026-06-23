// Chart telemetry types are derived from the Zod schemas in @/lib/schemas so the
// network boundary stays the single source of truth (no unvalidated `as` casts).
export type { CCVProfile, PowerReading, SOCSnapshot } from "@/lib/schemas";
