import { getDatabase } from "./db";
import Database from "better-sqlite3";

/**
 * GraphQL context created once per incoming request.
 *
 * Holds the database connection and any request-scoped services.
 * Resolvers receive this via the `context` argument.
 */
export interface GqlContext {
  db: Database.Database;
}

/**
 * Factory function invoked by express-graphql for every request.
 *
 * NOTE: DataLoader instances should ideally live here so that all
 * resolvers within a single request share the same loader and
 * batching works across the full query tree.  Currently loaders
 * are created in individual resolvers — see user and post resolvers.
 */
export function createContext(): GqlContext {
  const db = getDatabase();
  return { db };
}
