"use client";

import { CCVProfileSchema, Vehicle } from "@/lib/schemas";
import { CCVProfile } from "@/types/chart";
import { formatPower } from "@/utils/chartFormatters";
import {
  buildStaticCurve,
  effectiveCapacityKwh,
  powerAtSOC,
} from "@/utils/eta";
import { useCallback, useMemo } from "react";

import LineChart from "./LineChart";

function CCVChart({ vehicle }: { vehicle: Vehicle }) {
  const profileData = useMemo(() => {
    const curve = buildStaticCurve({
      capacityKwh: effectiveCapacityKwh(
        vehicle.capacityKwh,
        vehicle.chargerOutputW,
        vehicle.time20to80Min ?? null,
      ),
      chargerOutputW: vehicle.chargerOutputW,
      chargingEfficiency: vehicle.chargingEfficiency,
      time0to100Min: vehicle.time0to100Min ?? null,
      time0to80Min: vehicle.time0to80Min ?? null,
      time20to80Min: vehicle.time20to80Min ?? null,
      time20to100Min: vehicle.time20to100Min ?? null,
      packVoltageMaxV: vehicle.packVoltageMaxV ?? null,
      packCutoffCurrentMa: vehicle.packCutoffCurrentMa ?? null,
    });

    const data: CCVProfile[] = [];
    for (let soc = 0; soc <= 100; soc += 1) {
      const power = powerAtSOC(curve, soc);
      if (power !== null) {
        data.push({ soc, power: power * 1000 });
      }
    }

    return data;
  }, [
    vehicle.capacityKwh,
    vehicle.chargerOutputW,
    vehicle.chargingEfficiency,
    vehicle.time0to100Min,
    vehicle.time0to80Min,
    vehicle.time20to80Min,
    vehicle.time20to100Min,
    vehicle.packVoltageMaxV,
    vehicle.packCutoffCurrentMa,
  ]);

  const yExtractor = useCallback((p: CCVProfile) => p.power, []);

  const xExtractor = useCallback((p: CCVProfile) => {
    return `${Math.round(p.soc)}%`;
  }, []);

  const computeYDomain = useCallback(
    (points: CCVProfile[]): [number, number] => {
      if (points.length === 0) return [0, 1];
      let maxP = 0;
      for (const p of points) {
        if (p.power > maxP) maxP = p.power;
      }
      return [0, maxP];
    },
    [],
  );

  const yFormatter = useCallback((v: number, yMax: number) => {
    return formatPower(v, yMax);
  }, []);

  return (
    <LineChart<CCVProfile>
      schema={CCVProfileSchema}
      staticData={profileData}
      vehicleId={vehicle.id}
      yExtractor={yExtractor}
      timestampExtractor={xExtractor}
      yDomain={computeYDomain}
      lineColor="#3b82f6"
      yFormatter={yFormatter}
      minPointsForRender={2}
      ariaLabel="CC/CV charging profile"
      heightPx={140}
      messages={{
        loading: "Generating profile...",
        empty: "No profile data",
        waiting: "Insufficient data",
      }}
      fetchConfig={{ endpoint: "", pollingIntervalMs: 0 }}
    />
  );
}

export default CCVChart;
