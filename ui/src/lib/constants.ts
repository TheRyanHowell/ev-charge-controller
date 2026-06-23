// Speedometer gauge geometry
export const GAUGE_ARC_START_ANGLE_DEG = 135;
export const GAUGE_ARC_SPAN_DEG = 270;
export const GAUGE_FULL_ROTATION_DEG = 360;
export const RADIANS_PER_DEGREE = Math.PI / 180;
export const HIT_TOLERANCE_DEG = 6;
// Outer SVG viewBox dimensions: 300×280 (bottom 20px cropped for layout).
// The gauge circle is centered at (150,150) in the 300×300 inner coordinate space,
// so the gauge center lies at 53.6% of the outer viewBox height - not 50%.
export const GAUGE_VIEWBOX_W = 300;

// Power formatting
export const WATT_TO_KILO_WATT_THRESHOLD = 1000;
export const PERCENT_SCALE = 100;

// Timing
export const POLL_INTERVAL_MS = 5000;
export const TARGET_UPDATE_DEBOUNCE_MS = 300;
// Coalesces rapid arrow-key marker edits into a single persist.
export const KEYBOARD_COMMIT_DEBOUNCE_MS = 250;
export const ERROR_AUTO_DISMISS_MS = 5000;
export const TEMP_ERROR_FLASH_MS = 5000;

// Chart defaults
export const DEFAULT_CHART_HEIGHT_PX = 160;
export const MIN_POINTS_FOR_RENDER = 1;
