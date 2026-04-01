import Database from "better-sqlite3";
import path from "path";

const DB_PATH = process.env.DB_PATH || path.join(__dirname, "..", "data", "app.db");

let queryCount = 0;

/**
 * Open a connection to the SQLite database.
 * We use WAL mode for better concurrent read performance.
 */
export function getDatabase(): Database.Database {
  const db = new Database(DB_PATH);
  db.pragma("journal_mode = WAL");
  return db;
}

/**
 * Tracked query execution — wraps db.prepare().all() and increments a counter.
 * Used by the performance test to assert bounded query counts.
 */
export function trackedQuery(db: Database.Database, sql: string, ...params: any[]): any[] {
  queryCount++;
  return db.prepare(sql).all(...params);
}

/**
 * Tracked single-row query — wraps db.prepare().get().
 */
export function trackedQueryOne(db: Database.Database, sql: string, ...params: any[]): any {
  queryCount++;
  return db.prepare(sql).get(...params);
}

export function resetQueryCount(): void {
  queryCount = 0;
}

export function getQueryCount(): number {
  return queryCount;
}
