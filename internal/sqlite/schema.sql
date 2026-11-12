CREATE TABLE IF NOT EXISTS users (
 id TEXT PRIMARY KEY NOT NULL,
 name TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS accounts (
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 platform TEXT NOT NULL,
 handle TEXT NOT NULL,
 revision TEXT NOT NULL,
 profile TEXT CHECK(profile IS NULL OR json_valid(profile)),
 updated_at TEXT,
 last_error TEXT,
 PRIMARY KEY(user_id,platform)
);

-- Upgrade snapshots produced by the original shared Profile format.
UPDATE accounts SET profile = json_extract(profile,'$.details')
WHERE platform='codeforces' AND json_type(profile,'$.details')='object';
UPDATE accounts SET profile = json_set(json_remove(profile,'$.details'),
 '$.ratedContests',json_extract(profile,'$.details.ratedContests'),
 '$.latestContest',json_extract(profile,'$.details.latestContest'))
WHERE platform='atcoder' AND json_type(profile,'$.details')='object';
