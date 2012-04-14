DROP TABLE IF EXISTS result;
DROP TABLE IF EXISTS time_log;
DROP TABLE IF EXISTS start_time;
DROP TABLE IF EXISTS team;

CREATE TABLE team (
  id INTEGER PRIMARY KEY NOT NULL,
  number INTEGER NOT NULL UNIQUE,
  name TEXT NOT NULL UNIQUE
);

CREATE TABLE start_time (
  time TEXT NOT NULL -- HH:MM:SS
);

CREATE TABLE time_log (
  team_id INTEGER NOT NULL,
  time TEXT NOT NULL, -- HH:MM:SS (absolute time, not duration to ease input)
  UNIQUE (team_id, time), -- to avoid double submission
  FOREIGN KEY (team_id) REFERENCES team(id)
);

CREATE TABLE result (
  team_id INTEGER PRIMARY KEY NOT NULL,
  nb_lap INTEGER NOT NULL,
  time INTEGER NOT NULL, -- HH:MM:SS (absolute time, not duration)
  FOREIGN KEY (team_id) REFERENCES team(id)
);

CREATE TRIGGER result_generation AFTER INSERT ON time_log
BEGIN
  REPLACE INTO result
    SELECT team_id,
      count(1),
      max(time)
      FROM time_log GROUP BY team_id;
END;
