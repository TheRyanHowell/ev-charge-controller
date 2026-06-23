/**
 * CC/CV CHARGING CURVE - PHYSICS, MATHEMATICS, AND ALGORITHM
 *
 * ─── PLAIN-ENGLISH OVERVIEW ──────────────────────────────────────────────────
 *
 * Every lithium-ion battery charger follows a two-phase protocol:
 *
 *  1. CONSTANT CURRENT (CC) phase - "filling the tank quickly"
 *     The charger pushes a fixed current into the battery. Because battery
 *     voltage slowly rises as it fills, total power (P = V × I) also rises
 *     slightly. But modern chargers cap output at their rated wattage ceiling,
 *     so the power curve is effectively FLAT for the bulk of the charge
 *     (20–80% SOC in our model). Think of it as a fire hose running at full
 *     pressure.
 *
 *  2. CONSTANT VOLTAGE (CV) phase - "topping off carefully"
 *     When the battery reaches its max voltage, the charger holds voltage
 *     fixed and lets current decay naturally to prevent overcharging.
 *     Current (and therefore power) falls exponentially toward a cutoff
 *     threshold. Think of it as filling a glass to the brim - you slow
 *     down as it gets full to avoid spilling.
 *
 *  3. SOFT-START (0–20% SOC) - "waking up a depleted pack"
 *     Many BMS controllers ramp current gradually when a battery is very
 *     depleted, to avoid thermal shock. This creates a linear ramp from P0
 *     (start power) up to the CC plateau (P20 = P80).
 *
 * ─── MATHEMATICAL PROOF: CV PHASE IS EXACTLY LINEAR IN SOC ──────────────────
 *
 * In the CV phase, current decays exponentially with time:
 *   I(t) = I_max · exp(-t / τ)        where τ is the pack time constant (s)
 *
 * Power at any time (V_max is held constant by the charger):
 *   P(t) = V_max · I(t) = P_max · exp(-t / τ)
 *
 * Cumulative CV energy delivered from t=0:
 *   E(t) = ∫₀ᵗ P(t') dt' = P_max · τ · (1 − exp(−t / τ))
 *
 * Solving for exp(−t/τ) and substituting back:
 *   P = P_max − E / τ
 *
 * Since SOC ∝ E (energy delivered ÷ pack capacity), P is EXACTLY LINEAR in
 * SOC during the CV phase. A straight line on a P-vs-SOC chart is not an
 * approximation - it is the precise mathematical projection of exponential
 * current decay onto the SOC axis.
 *
 * ─── DERIVING P100 FROM ENERGY CONSERVATION ──────────────────────────────────
 *
 * The CV phase covers 20% of usable capacity (80→100% SOC). We model power
 * as a straight line from P80 (CV entry = P_max) to P100 (charge termination).
 *
 * Energy under a straight line from P80 to P100 over time t_cv:
 *   E_cv = (P80 + P100) / 2 · t_cv
 *
 * Setting E_cv = 0.20 · capacityKwh and solving for P100:
 *   P100 = 2 · (capacityKwh · 0.20 / t_cv_hours) − P80
 *
 * ─── DERIVING P100 FROM PACK SPECS (FALLBACK) ────────────────────────────────
 *
 * When no CV-segment time data is available, P100 can be computed from the
 * pack's electrical end-of-charge conditions if the user has supplied them:
 *   P100 = packVoltageMaxV × packCutoffCurrentMa / 1_000_000  (kW)
 *          ↑ pack voltage (V)   ↑ pack cutoff current (mA → A / 1000 → kW / 1000)
 *
 * Example - Maeving RM1 (LG MJ1 cells, 14S × 12P, standard charge spec):
 *   packVoltageMaxV     = 14 × 4.20 V = 58.8 V
 *   packCutoffCurrentMa = 50 mA/cell × 12P = 600 mA
 *   P100 = 58.8 × 600 / 1_000_000 = 0.0353 kW ≈ 35 W
 *   (time-based formula gives ≈33 W - 6% gap from the 80% SOC boundary
 *   approximation; both are physically valid representations)
 *
 * IMPORTANT: time-based P100 takes priority over physics-based P100 because
 * the time formula is self-consistent with the manufacturer's spec charge
 * times. Physics-based is used only when no CV time data is available.
 *
 * ─── EFFECTIVE CAPACITY VS. NOMINAL CAPACITY ─────────────────────────────────
 *
 * Nominal capacity is the cell's rated energy (e.g., "2 kWh" on the spec
 * sheet). The BMS allows only a portion - "usable" or "effective" capacity -
 * to avoid degrading cells at the extremes. effectiveCapacityKwh() derives
 * this from the 20–80% charge time: those 60 percentage-points are charged
 * at full charger output, so:
 *   effective_kWh = (t20to80_hours × charger_kW) / 0.60
 *
 * Using nominal instead of effective capacity causes soft-start power (P0)
 * to be wildly wrong (≈8 W instead of ≈238 W for RM1). Always use
 * effectiveCapacityKwh() when calling buildStaticCurve.
 *
 * ─── MISSING-DATA FALLBACK HIERARCHY ─────────────────────────────────────────
 *
 *  P20 / P80:  derived from t20to80Min → falls back to charger rated output
 *  P100:       1. time-based (t20to100 − t20to80, or t0to100 − t0to80)
 *              2. physics-based (packVoltageMaxV × packCutoffCurrentMa / 1e6)
 *              3. 0 kW (taper to zero - conservative safe default)
 *  P0:         derived from t0to80Min − t20to80Min → falls back to P20 × 0.5
 *
 * ─── REAL-TIME ETA CALIBRATION ───────────────────────────────────────────────
 *
 * dynamicEffectiveCapacityKwh() improves ETA during an active session by
 * replacing the spec-based capacity estimate with one derived from observed
 * energy delivery: actual_cap = energy_added_kWh / delta_SOC_fraction.
 * This cancels out BMS tolerance, cell aging, and temperature effects.
 * It only activates when the vehicle is between 20–80% SOC and enough data
 * has been collected (≥10% SOC change, ≥0.1 kWh observed). Otherwise it
 * falls back to the spec-based effectiveCapacityKwh().
 */

// Charging curve thresholds - must match backend models/constants.go
const SOC80Percent = 80.0;
const SOC20Percent = 20.0;
const PreCCPhaseSize = 20.0;
const CVPhaseSize = 20.0;
const UsableCapacityDivisor = 0.6;
const CapacityValidationMinRatio = 0.5;
const CapacityValidationMaxRatio = 1.05;
const DefaultChargingEfficiency = 0.8;

// Curve derivation constants
const defaultCRate = 0.5; // assumed C-rate when charger output is unknown
const ccPhasePercent = 0.6; // 20–80% SOC = 60% of pack capacity
const segmentPercent = 0.2; // each 20-SOC-point segment = 20% of capacity
const defaultSoftStartRatio = 0.5; // P0 = 50% of P_max when no pre-CC time data

// Dynamic capacity calibration thresholds
const minObservedEnergyKwh = 0.1; // require ≥100 Wh of observed delivery
const minObservedSocDelta = 10.0; // require ≥10% SOC change
const maxCapacityCalibrationRatio = 1.5; // reject if > 1.5× spec capacity
const minCapacityCalibrationRatio = 0.5; // reject if < 0.5× spec capacity

// Integration step size in kWh for ETA calculation
const integrationStepKwh = 0.1;

interface ChargingCurve {
  P0: number;
  P20: number;
  P80: number;
  P100: number;
}

interface CurveConfig {
  capacityKwh: number;
  chargerOutputW: number;
  chargingEfficiency: number;
  time0to80Min: number | null;
  time0to100Min: number | null;
  time20to80Min: number | null;
  time20to100Min: number | null;
  // Optional pack electrical specs for physics-based P100 fallback.
  // Used only when no CV-segment time data is available.
  // P100 = packVoltageMaxV (V) × packCutoffCurrentMa (mA) / 1_000_000 (kW)
  packVoltageMaxV?: number | null;
  packCutoffCurrentMa?: number | null;
}

/**
 * PowerAtSOC returns the interpolated power (kW) at the given SOC percentage.
 * Returns null if SOC is out of bounds or no segments are defined.
 */
export function powerAtSOC(curve: ChargingCurve, soc: number): number | null {
  if (soc < 0 || soc > 100) return null;

  const { P0, P20, P80, P100 } = curve;
  if (!(P0 > 0 || P20 > 0 || P80 > 0 || P100 > 0)) return null;

  if (soc <= 20) {
    if (P20 === 0) return P0 > 0 ? P0 : null;
    if (P0 === 0) return P20;
    return P0 + (P20 - P0) * (soc / 20);
  } else if (soc <= 80) {
    if (P80 === 0) return P20 > 0 ? P20 : null;
    if (P20 === 0) return P80;
    return P20 + (P80 - P20) * ((soc - 20) / 60);
  } else {
    if (P100 === 0) return P80 > 0 ? P80 : null;
    if (P80 === 0) return P100;
    return P80 + (P100 - P80) * ((soc - 80) / 20);
  }
}

/**
 * SmoothingPower applies a 1-point moving average to smooth segment-boundary jumps.
 */
function smoothingPower(prev: number, current: number): number {
  if (prev <= 0) return current;
  return (prev + current) / 2;
}

/**
 * integrationLoop performs step-wise kWh integration.
 * Returns total time in minutes.
 */
function integrationLoop(
  curve: ChargingCurve,
  currentKwh: number,
  targetKwh: number,
  capacityKwh: number,
): number {
  let totalMin = 0;
  let kwh = currentKwh;
  let prevPower = 0;

  while (kwh < targetKwh) {
    const soc = (kwh / capacityKwh) * 100;
    const powerKw = powerAtSOC(curve, soc);
    if (powerKw === null || powerKw <= 0) return 0;

    const smoothed = smoothingPower(prevPower, powerKw);
    prevPower = smoothed;

    let actualStep = integrationStepKwh;
    if (kwh + integrationStepKwh > targetKwh) {
      actualStep = targetKwh - kwh;
    }
    totalMin += (actualStep / smoothed) * 60;

    kwh += integrationStepKwh;
    if (kwh > targetKwh) kwh = targetKwh;
  }

  return totalMin;
}

/**
 * IntegrationETA computes the estimated time (minutes) to charge from currentKwh to
 * targetKwh using step-wise integration over a charging curve.
 *
 * When time0to20Min and cvTimeMin are non-null, uses three-phase mode:
 *  - Pre-CC phase (0-20%): static time from manufacturer specs
 *  - CC phase (20-80%): dynamic curve integration
 *  - CV phase (80-100%): static time from manufacturer specs
 *
 * Returns 0 if target is already reached, capacity is invalid, or curve has no data.
 */
export function integrationETA(
  curve: ChargingCurve,
  currentKwh: number,
  targetKwh: number,
  capacityKwh: number,
  time0to20Min: number | null,
  cvTimeMin: number | null,
): number {
  if (targetKwh <= currentKwh || capacityKwh <= 0) return 0;

  const currentSOC = (currentKwh / capacityKwh) * 100;
  const targetSOC = (targetKwh / capacityKwh) * 100;

  const cvBoundaryKwh = (SOC80Percent / 100) * capacityKwh;
  const ccBoundaryKwh = (SOC20Percent / 100) * capacityKwh;

  let totalMin = 0;

  // Pre-CC phase: 0-20% static time from manufacturer specs
  if (time0to20Min != null && currentSOC < SOC20Percent) {
    let preMin: number;
    if (targetSOC >= SOC20Percent) {
      preMin = (time0to20Min * (SOC20Percent - currentSOC)) / PreCCPhaseSize;
    } else {
      preMin = (time0to20Min * (targetSOC - currentSOC)) / PreCCPhaseSize;
    }
    totalMin += preMin;
  }

  // CC phase: integrate from max(current, dynamic start) to min(target, dynamic end)
  let ccStartKwh: number;
  let ccEndKwh: number;

  if (time0to20Min != null && currentSOC < SOC20Percent) {
    ccStartKwh = ccBoundaryKwh;
  } else {
    ccStartKwh = currentKwh;
  }

  if (cvTimeMin != null && targetSOC > SOC80Percent) {
    ccEndKwh = cvBoundaryKwh;
  } else {
    ccEndKwh = targetKwh;
  }

  if (ccEndKwh > ccStartKwh) {
    totalMin += integrationLoop(curve, ccStartKwh, ccEndKwh, capacityKwh);
  }

  // CV phase: 80-100% static time from manufacturer specs
  if (cvTimeMin != null && targetSOC > SOC80Percent) {
    let cvMin: number;
    if (currentSOC < SOC80Percent) {
      cvMin = (cvTimeMin * (targetSOC - SOC80Percent)) / CVPhaseSize;
    } else {
      cvMin = (cvTimeMin * (targetSOC - currentSOC)) / CVPhaseSize;
    }
    totalMin += cvMin;
  }

  return Math.ceil(totalMin);
}

/**
 * ComputeCVTimeMin derives the CV (constant voltage) phase time in minutes.
 * Returns null when there is insufficient data.
 */
export function computeCVTimeMin(
  t0100: number | null,
  t080: number | null,
  t20100: number | null,
  t2080: number | null,
): number | null {
  if (t20100 != null && t2080 != null && t2080 > 0 && t20100 > t2080) {
    return t20100 - t2080;
  }
  if (t0100 != null && t080 != null && t080 > 0 && t0100 > t080) {
    return t0100 - t080;
  }
  return null;
}

/**
 * ComputeTime0to20Min derives the pre-CC (0-20%) phase time in minutes.
 * Returns null when there is insufficient data.
 */
export function computeTime0to20Min(
  t0100: number | null,
  t080: number | null,
  t20100: number | null,
  t2080: number | null,
): number | null {
  if (t080 != null && t2080 != null && t080 > 0 && t2080 > 0 && t080 > t2080) {
    return t080 - t2080;
  }
  if (t0100 != null && t20100 != null && t20100 > 0 && t0100 > t20100) {
    return t0100 - t20100;
  }
  return null;
}

// --- Curve derivation helpers ---

function deriveP20(capacityKwh: number, t2080: number | null): number {
  if (t2080 == null || t2080 <= 0) return 0;
  const energy = capacityKwh * ccPhasePercent;
  const hours = t2080 / 60;
  if (hours <= 0) return 0;
  return energy / hours;
}

// deriveP100 uses energy conservation to find P100 from the CV-segment time.
// Physics: (P80 + P100)/2 × t_cv = capacity × 0.20 → P100 = 2·E_cv/t_cv − P80.
// Prefers t20-based CV time; falls back to t0-based when t20 data is absent.
function deriveP100(
  capacityKwh: number,
  t20100: number | null,
  t2080: number | null,
  t0100: number | null,
  t080: number | null,
  p80: number,
): number {
  let cvTimeMin: number | null = null;
  if (t20100 != null && t2080 != null && t2080 > 0 && t20100 > t2080) {
    cvTimeMin = t20100 - t2080;
  } else if (t0100 != null && t080 != null && t080 > 0 && t0100 > t080) {
    cvTimeMin = t0100 - t080;
  }
  if (cvTimeMin == null || cvTimeMin <= 0) return 0;

  const energy = capacityKwh * segmentPercent;
  const hours = cvTimeMin / 60;
  if (hours <= 0) return 0;

  if (p80 > 0) {
    const p100 = (2 * energy) / hours - p80;
    return p100 > 0 ? p100 : 0;
  }
  return energy / hours;
}

function deriveP0(
  capacityKwh: number,
  t080: number | null,
  t2080: number | null,
  t0100: number | null,
  t20100: number | null,
  p20: number,
): number {
  let t0to20 = 0;

  if (t080 != null && t080 > 0 && t2080 != null && t2080 > 0) {
    t0to20 = t080 - t2080;
  } else if (t0100 != null && t0100 > 0 && t20100 != null && t20100 > 0) {
    t0to20 = t0100 - t20100;
  }

  if (t0to20 <= 0) return 0;

  const hours = t0to20 / 60;
  const energy = capacityKwh * segmentPercent;
  const p0 = (2 * energy) / hours - p20;
  if (p0 <= 0) return energy / hours;
  return p0;
}

// deriveP100FromPackSpecs computes the end-of-charge power from known pack electrical specs.
// P = V_pack_max × I_cutoff_total. Both inputs are in their natural SI-prefix units
// (volts and milliamps) so the divisor is 1_000_000 to arrive at kilowatts.
function deriveP100FromPackSpecs(
  packVoltageMaxV: number | null | undefined,
  packCutoffCurrentMa: number | null | undefined,
): number {
  if (!packVoltageMaxV || !packCutoffCurrentMa) return 0;
  return (packVoltageMaxV * packCutoffCurrentMa) / 1_000_000;
}

// enforceMonotonicity ensures physical ordering: soft-start ≤ CC ≤ CC, CV always declines.
// P0 ≤ P20: soft-start cannot exceed the CC plateau it ramps toward.
// P20 = P80: CC phase is flat (charger at rated ceiling throughout 20–80%).
// P100 ≤ P80: CV phase always tapers; power can only fall during constant-voltage.
function enforceMonotonicity(c: ChargingCurve): void {
  if (c.P0 > c.P20) c.P0 = c.P20;
  if (c.P80 > c.P20) c.P80 = c.P20;
  if (c.P100 > c.P80) c.P100 = c.P80;
}

// enforceChargerLimit clamps all curve points to the charger's rated output.
// Skip entirely when chargerOutputW is zero - the caller already used defaultCRate.
function enforceChargerLimit(c: ChargingCurve, chargerOutputW: number): void {
  const maxPowerKw = chargerOutputW / 1000;
  if (maxPowerKw <= 0) return;
  if (c.P0 > maxPowerKw) c.P0 = maxPowerKw;
  if (c.P20 > maxPowerKw) c.P20 = maxPowerKw;
  if (c.P80 > maxPowerKw) c.P80 = maxPowerKw;
  if (c.P100 > maxPowerKw) c.P100 = maxPowerKw;
}

/**
 * buildStaticCurve constructs a ChargingCurve from manufacturer-rated specs.
 * No real-time data enters here - this is a pure static representation derived
 * from charge-time specifications and optional pack electrical parameters.
 *
 * P20 = P80 (CC phase is flat); P100 is derived from energy conservation over
 * the CV segment; P0 is the soft-start entry point. All four points fall through
 * the same logic regardless of which inputs are available, with graceful fallbacks.
 */
export function buildStaticCurve(cfg: CurveConfig): ChargingCurve {
  // Maximum power available: charger rated output, or capacity × default C-rate
  // when charger output is unknown (e.g., vehicle configured without charger spec).
  const chargerKw = cfg.chargerOutputW / 1000;
  const pMax = chargerKw > 0 ? chargerKw : cfg.capacityKwh * defaultCRate;

  // CC phase - flat at charger ceiling (P20 = P80).
  // Derived from t20to80Min (energy ÷ time); capped at charger max.
  const pCC = Math.min(
    deriveP20(cfg.capacityKwh, cfg.time20to80Min) || pMax,
    pMax,
  );

  // CV phase endpoint - P100 is where charging terminates.
  // Priority: time-based formula (self-consistent with spec times) >
  //           physics-based (pack voltage × cutoff current) > 0 (taper to zero).
  const p100Time = deriveP100(
    cfg.capacityKwh,
    cfg.time20to100Min,
    cfg.time20to80Min,
    cfg.time0to100Min,
    cfg.time0to80Min,
    pCC,
  );
  const p100 =
    p100Time > 0
      ? p100Time
      : deriveP100FromPackSpecs(cfg.packVoltageMaxV, cfg.packCutoffCurrentMa);

  // Soft-start - linear ramp from P0 to P20.
  // Derived from t0to80Min − t20to80Min; falls back to 50% of P_max when absent.
  const p0Raw = deriveP0(
    cfg.capacityKwh,
    cfg.time0to80Min,
    cfg.time20to80Min,
    cfg.time0to100Min,
    cfg.time20to100Min,
    pCC,
  );
  const p0 = p0Raw > 0 ? p0Raw : pCC * defaultSoftStartRatio;

  const c: ChargingCurve = { P0: p0, P20: pCC, P80: pCC, P100: p100 };
  enforceMonotonicity(c);
  enforceChargerLimit(c, cfg.chargerOutputW);

  return c;
}

/**
 * effectiveCapacityKwh returns the BMS usable capacity if derivable, otherwise nominal.
 */
export function effectiveCapacityKwh(
  capacityKwh: number,
  chargerOutputW: number,
  time20to80Min: number | null,
): number {
  if (time20to80Min == null || time20to80Min <= 0 || chargerOutputW <= 0) {
    return capacityKwh;
  }
  const chargerKw = chargerOutputW / 1000;
  const hours = time20to80Min / 60;
  const usable = (hours * chargerKw) / UsableCapacityDivisor;
  if (
    usable > capacityKwh * CapacityValidationMaxRatio ||
    usable < capacityKwh * CapacityValidationMinRatio
  ) {
    return capacityKwh;
  }
  return usable;
}

/**
 * dynamicEffectiveCapacityKwh improves the capacity estimate during an active session
 * by using observed energy delivery instead of (or in addition to) spec timing.
 *
 * How it works: between 20–80% SOC the charger runs at rated power, so energy
 * delivered is proportional to SOC change. We can therefore measure actual usable
 * capacity directly: cap = energy_added_kWh / (delta_SOC / 100).
 *
 * Only activates when: both SOC endpoints are within the CC range (20–80%), the
 * SOC change is ≥10%, and at least 0.1 kWh has been observed. Falls back to the
 * spec-based effectiveCapacityKwh() otherwise.
 */
export function dynamicEffectiveCapacityKwh(
  specCapacityKwh: number,
  chargerOutputW: number,
  time20to80Min: number | null,
  energyAddedKwh: number | null | undefined,
  startPercent: number | null | undefined,
  currentPercent: number | null | undefined,
): number {
  const specCap = effectiveCapacityKwh(
    specCapacityKwh,
    chargerOutputW,
    time20to80Min,
  );

  if (
    energyAddedKwh == null ||
    startPercent == null ||
    currentPercent == null ||
    energyAddedKwh < minObservedEnergyKwh
  ) {
    return specCap;
  }

  const deltaSoc = currentPercent - startPercent;
  if (
    deltaSoc < minObservedSocDelta ||
    startPercent < SOC20Percent ||
    currentPercent > SOC80Percent
  ) {
    return specCap;
  }

  const observed = energyAddedKwh / (deltaSoc / 100);
  if (
    observed > specCapacityKwh * maxCapacityCalibrationRatio ||
    observed < specCapacityKwh * minCapacityCalibrationRatio
  ) {
    return specCap;
  }

  return observed;
}

/**
 * hasCurveData returns true if the vehicle has any data for curve construction.
 */
export function hasCurveData(cfg: CurveConfig): boolean {
  return (
    cfg.time20to80Min != null ||
    cfg.time0to80Min != null ||
    cfg.time0to100Min != null ||
    cfg.time20to100Min != null ||
    cfg.chargerOutputW > 0 ||
    cfg.capacityKwh > 0
  );
}

/**
 * calculateETA computes the estimated time (minutes) to charge from currentPercent
 * to targetPercent using the vehicle's manufacturer specs.
 *
 * Pass energyAddedKwh and startPercent from an active session to enable
 * real-time capacity calibration via dynamicEffectiveCapacityKwh.
 *
 * Returns null if no estimate can be computed.
 */
export function calculateETA({
  currentPercent,
  targetPercent,
  capacityKwh,
  chargerOutputW,
  chargingEfficiency,
  time0to80Min,
  time0to100Min,
  time20to80Min,
  time20to100Min,
  energyAddedKwh,
  startPercent,
}: {
  currentPercent: number;
  targetPercent: number;
  capacityKwh: number;
  chargerOutputW: number;
  chargingEfficiency: number;
  time0to80Min: number | null;
  time0to100Min: number | null;
  time20to80Min: number | null;
  time20to100Min: number | null;
  energyAddedKwh?: number | null;
  startPercent?: number | null;
}): number | null {
  if (currentPercent >= targetPercent) return null;
  if (capacityKwh <= 0) return null;

  const eff =
    chargingEfficiency > 0 ? chargingEfficiency : DefaultChargingEfficiency;
  const effectiveCap = dynamicEffectiveCapacityKwh(
    capacityKwh,
    chargerOutputW,
    time20to80Min,
    energyAddedKwh,
    startPercent,
    currentPercent,
  );

  const cfg: CurveConfig = {
    capacityKwh: effectiveCap,
    chargerOutputW,
    chargingEfficiency: eff,
    time0to80Min,
    time0to100Min,
    time20to80Min,
    time20to100Min,
  };

  if (!hasCurveData(cfg)) return null;

  const curve = buildStaticCurve(cfg);
  const currentKwh = (effectiveCap * currentPercent) / 100;
  const targetKwh = (effectiveCap * targetPercent) / 100;

  const cvTime = computeCVTimeMin(
    time0to100Min,
    time0to80Min,
    time20to100Min,
    time20to80Min,
  );
  const time0to20 = computeTime0to20Min(
    time0to100Min,
    time0to80Min,
    time20to100Min,
    time20to80Min,
  );

  const eta = integrationETA(
    curve,
    currentKwh,
    targetKwh,
    effectiveCap,
    time0to20,
    cvTime,
  );

  return eta > 0 ? eta : null;
}
