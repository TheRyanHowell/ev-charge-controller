-- Seed the global vehicle model catalog.
-- pack_voltage_max_v / pack_cutoff_current_ma: verified from datasheet + time-data cross-check.
--
-- RM1 (LG MJ1, 14S×12P, standard spec): 14×4.20V=58.8V, 12×50mA=600mA
-- RM1S (LG M50LT, 14S×11P × 1 battery, LEV-adjusted spec): 14×4.10V=57.4V, 1×11×250mA=2750mA
-- RM2 (LG M50LT, 14S×11P × 2 batteries parallel, LEV-adjusted spec): 14×4.10V=57.4V, 2×11×250mA=5500mA
INSERT OR IGNORE INTO vehicle_models (id, name, capacity_kwh, charger_output_w, charging_efficiency, time_0_100_min, time_0_80_min, time_20_80_min, time_20_100_min, pack_voltage_max_v, pack_cutoff_current_ma, range_min_mi, range_max_mi) VALUES
  ('rm1',  'Maeving RM1',  2.026, 600,  0.8, 250, 175, 95,  155, 58.8, 600,  29, 40),
  ('rm1s', 'Maeving RM1S', 5.46,  1200, 0.8, 360, 0,   150, 240, 57.4, 2750, 52, 80),
  ('rm2',  'Maeving RM2',  5.46,  1200, 0.8, 360, 0,   150, 240, 57.4, 5500, 52, 80);
