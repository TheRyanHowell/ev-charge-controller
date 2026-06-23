CREATE TABLE power_readings_backup AS
  SELECT id, session_id, timestamp, voltage, current, power, energy_kwh
  FROM power_readings;
DROP TABLE power_readings;
ALTER TABLE power_readings_backup RENAME TO power_readings;
