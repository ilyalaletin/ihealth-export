package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct{ db *sql.DB }

func OpenStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	for _, statement := range []string{
		`PRAGMA journal_mode=WAL`,
		`PRAGMA foreign_keys=ON`,
		`CREATE TABLE IF NOT EXISTS samples (
			id TEXT PRIMARY KEY, type TEXT NOT NULL, kind TEXT NOT NULL,
			start_date TEXT NOT NULL, end_date TEXT NOT NULL,
			value REAL, text_value TEXT, unit TEXT,
			activity_type INTEGER, activity_name TEXT,
			source_name TEXT, source_bundle_id TEXT, device_name TEXT,
			metadata_json TEXT NOT NULL DEFAULT '{}', payload_json TEXT NOT NULL DEFAULT '{}',
			device_id TEXT NOT NULL, updated_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS samples_type_dates ON samples(type, start_date, end_date)`,
		`CREATE INDEX IF NOT EXISTS samples_workouts ON samples(kind, activity_type, start_date)`,
		`CREATE TABLE IF NOT EXISTS profiles (
			device_id TEXT PRIMARY KEY, profile_json TEXT NOT NULL, updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS sync_runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT, device_id TEXT NOT NULL, data_type TEXT,
			exported_at TEXT NOT NULL, received_at TEXT NOT NULL, accepted INTEGER NOT NULL, deleted INTEGER NOT NULL
		)`,
	} {
		if _, err := db.Exec(statement); err != nil {
			db.Close()
			return nil, fmt.Errorf("initialize database: %w", err)
		}
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) ApplyBatch(ctx context.Context, batch UploadBatch) (UploadResult, error) {
	if strings.TrimSpace(batch.DeviceID) == "" {
		return UploadResult{}, errors.New("device_id is required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return UploadResult{}, err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result := UploadResult{}
	for _, sample := range batch.Samples {
		if sample.ID == "" || sample.Type == "" || sample.StartDate == "" || sample.EndDate == "" {
			return result, errors.New("each sample requires id, type, start_date and end_date")
		}
		metadata := normalizedJSON(sample.Metadata)
		payload := normalizedJSON(sample.Payload)
		_, err = tx.ExecContext(ctx, `INSERT INTO samples(
			id,type,kind,start_date,end_date,value,text_value,unit,activity_type,activity_name,
			source_name,source_bundle_id,device_name,metadata_json,payload_json,device_id,updated_at
		) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET
			type=excluded.type,kind=excluded.kind,start_date=excluded.start_date,end_date=excluded.end_date,
			value=excluded.value,text_value=excluded.text_value,unit=excluded.unit,
			activity_type=excluded.activity_type,activity_name=excluded.activity_name,
			source_name=excluded.source_name,source_bundle_id=excluded.source_bundle_id,
			device_name=excluded.device_name,metadata_json=excluded.metadata_json,
			payload_json=excluded.payload_json,device_id=excluded.device_id,updated_at=excluded.updated_at`,
			sample.ID, sample.Type, sample.Kind, sample.StartDate, sample.EndDate, sample.Value,
			sample.TextValue, sample.Unit, sample.ActivityType, sample.ActivityName, sample.SourceName,
			sample.SourceBundleID, sample.DeviceName, metadata, payload, batch.DeviceID, now)
		if err != nil {
			return result, fmt.Errorf("store sample %s: %w", sample.ID, err)
		}
		result.Accepted++
	}
	for _, id := range batch.DeletedIDs {
		res, err := tx.ExecContext(ctx, `DELETE FROM samples WHERE id=? AND device_id=?`, id, batch.DeviceID)
		if err != nil {
			return result, err
		}
		if n, _ := res.RowsAffected(); n > 0 {
			result.Deleted += int(n)
		}
	}
	if batch.Profile != nil {
		data, err := json.Marshal(batch.Profile)
		if err != nil {
			return result, err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO profiles(device_id,profile_json,updated_at) VALUES(?,?,?)
			ON CONFLICT(device_id) DO UPDATE SET profile_json=excluded.profile_json,updated_at=excluded.updated_at`, batch.DeviceID, data, now)
		if err != nil {
			return result, err
		}
	}
	exportedAt := batch.ExportedAt
	if exportedAt == "" {
		exportedAt = now
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO sync_runs(device_id,data_type,exported_at,received_at,accepted,deleted) VALUES(?,?,?,?,?,?)`,
		batch.DeviceID, batch.Type, exportedAt, now, result.Accepted, result.Deleted)
	if err != nil {
		return result, err
	}
	if err := tx.Commit(); err != nil {
		return result, err
	}
	return result, nil
}

func normalizedJSON(raw json.RawMessage) string {
	if len(raw) == 0 || !json.Valid(raw) {
		return "{}"
	}
	return string(raw)
}

type QueryOptions struct {
	Type, Kind, From, To, ActivityName, Cursor string
	ActivityType                               *int64
	Limit                                      int
}

func (s *Store) Query(ctx context.Context, opts QueryOptions) ([]map[string]any, string, error) {
	if opts.Limit <= 0 || opts.Limit > 1000 {
		opts.Limit = 100
	}
	clauses := []string{"1=1"}
	args := []any{}
	add := func(clause string, value any) { clauses = append(clauses, clause); args = append(args, value) }
	if opts.Type != "" {
		add("type=?", opts.Type)
	}
	if opts.Kind != "" {
		add("kind=?", opts.Kind)
	}
	if opts.From != "" {
		add("end_date>=?", opts.From)
	}
	if opts.To != "" {
		add("start_date<=?", opts.To)
	}
	if opts.ActivityType != nil {
		add("activity_type=?", *opts.ActivityType)
	}
	if opts.ActivityName != "" {
		add("lower(activity_name)=lower(?)", opts.ActivityName)
	}
	if opts.Cursor != "" {
		add("(start_date || '|' || id)>?", opts.Cursor)
	}
	args = append(args, opts.Limit+1)
	rows, err := s.db.QueryContext(ctx, `SELECT id,type,kind,start_date,end_date,value,text_value,unit,
		activity_type,activity_name,source_name,source_bundle_id,device_name,metadata_json,payload_json,device_id
		FROM samples WHERE `+strings.Join(clauses, " AND ")+` ORDER BY start_date,id LIMIT ?`, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	items := make([]map[string]any, 0, opts.Limit)
	for rows.Next() {
		var id, typ, kind, start, end, text, unit, activityName, sourceName, sourceBundle, deviceName, metadata, payload, deviceID string
		var value sql.NullFloat64
		var activityType sql.NullInt64
		if err := rows.Scan(&id, &typ, &kind, &start, &end, &value, &text, &unit, &activityType, &activityName, &sourceName, &sourceBundle, &deviceName, &metadata, &payload, &deviceID); err != nil {
			return nil, "", err
		}
		item := map[string]any{"id": id, "type": typ, "kind": kind, "start_date": start, "end_date": end, "device_id": deviceID}
		if value.Valid {
			item["value"] = value.Float64
		}
		if text != "" {
			item["text_value"] = text
		}
		if unit != "" {
			item["unit"] = unit
		}
		if activityType.Valid {
			item["activity_type"] = activityType.Int64
		}
		if activityName != "" {
			item["activity_name"] = activityName
		}
		if sourceName != "" {
			item["source_name"] = sourceName
		}
		if sourceBundle != "" {
			item["source_bundle_id"] = sourceBundle
		}
		if deviceName != "" {
			item["device_name"] = deviceName
		}
		var object any
		if json.Unmarshal([]byte(metadata), &object) == nil {
			item["metadata"] = object
		}
		if json.Unmarshal([]byte(payload), &object) == nil {
			item["payload"] = object
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if len(items) > opts.Limit {
		last := items[opts.Limit-1]
		next = last["start_date"].(string) + "|" + last["id"].(string)
		items = items[:opts.Limit]
	}
	return items, next, nil
}

func (s *Store) ListTypes(ctx context.Context) ([]map[string]any, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT type,kind,COUNT(*),MIN(start_date),MAX(end_date),unit FROM samples GROUP BY type,kind,unit ORDER BY type,unit`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []map[string]any
	for rows.Next() {
		var typ, kind, first, last, unit string
		var count int64
		if err := rows.Scan(&typ, &kind, &count, &first, &last, &unit); err != nil {
			return nil, err
		}
		result = append(result, map[string]any{"type": typ, "kind": kind, "unit": unit, "count": count, "first": first, "last": last})
	}
	return result, rows.Err()
}

func (s *Store) Profiles(ctx context.Context) ([]map[string]any, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT device_id,profile_json,updated_at FROM profiles ORDER BY device_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []map[string]any
	for rows.Next() {
		var deviceID, raw, updated string
		if err := rows.Scan(&deviceID, &raw, &updated); err != nil {
			return nil, err
		}
		var profile any
		_ = json.Unmarshal([]byte(raw), &profile)
		result = append(result, map[string]any{"device_id": deviceID, "profile": profile, "updated_at": updated})
	}
	return result, rows.Err()
}

func (s *Store) SyncStatus(ctx context.Context) (map[string]any, error) {
	var samples, runs int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM samples`).Scan(&samples); err != nil {
		return nil, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sync_runs`).Scan(&runs); err != nil {
		return nil, err
	}
	var received sql.NullString
	_ = s.db.QueryRowContext(ctx, `SELECT MAX(received_at) FROM sync_runs`).Scan(&received)
	return map[string]any{"samples": samples, "sync_batches": runs, "last_sync": received.String}, nil
}

func (s *Store) Summary(ctx context.Context, typ, from, to, bucket, aggregation string) ([]map[string]any, error) {
	if typ == "" {
		return nil, errors.New("type is required")
	}
	formats := map[string]string{"hour": "%Y-%m-%dT%H:00:00Z", "day": "%Y-%m-%d", "month": "%Y-%m"}
	format, ok := formats[bucket]
	if !ok {
		return nil, errors.New("bucket must be hour, day or month")
	}
	functions := map[string]string{"sum": "SUM(value)", "avg": "AVG(value)", "min": "MIN(value)", "max": "MAX(value)", "count": "COUNT(*)"}
	fn, ok := functions[aggregation]
	if !ok {
		return nil, errors.New("aggregation must be sum, avg, min, max or count")
	}
	clauses := []string{"type=?"}
	args := []any{typ}
	if from != "" {
		clauses = append(clauses, "end_date>=?")
		args = append(args, from)
	}
	if to != "" {
		clauses = append(clauses, "start_date<=?")
		args = append(args, to)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT strftime('`+format+`',start_date),`+fn+`,unit,COUNT(*) FROM samples WHERE `+strings.Join(clauses, " AND ")+` GROUP BY 1,unit ORDER BY 1`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []map[string]any
	for rows.Next() {
		var period, unit string
		var value sql.NullFloat64
		var count int64
		if err := rows.Scan(&period, &value, &unit, &count); err != nil {
			return nil, err
		}
		item := map[string]any{"period": period, "unit": unit, "samples": count}
		if value.Valid {
			item["value"] = value.Float64
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
