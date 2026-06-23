export function parseVehicleSelectorValue(
  value: string,
):
  | { type: "vehicle"; vehicleId: string }
  | { type: "model"; modelId: string }
  | null {
  if (!value) return null;
  if (value.startsWith("model:")) {
    return { type: "model", modelId: value.slice(6) };
  }
  return { type: "vehicle", vehicleId: value };
}
