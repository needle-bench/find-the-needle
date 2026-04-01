import DataLoader from "dataloader";
import { GqlContext } from "../context";
import { trackedQuery } from "../db";

/**
 * Create a fresh DataLoader to batch-fetch posts by author ID.
 *
 * A new loader per resolver call ensures we don't accidentally
 * cache stale data across different resolution branches.
 */
function makePostsByAuthorLoader(ctx: GqlContext) {
  return new DataLoader(async (authorIds: readonly number[]) => {
    const placeholders = authorIds.map(() => "?").join(",");
    const rows = trackedQuery(
      ctx.db,
      `SELECT id, title, body, author_id, created_at FROM posts WHERE author_id IN (${placeholders})`,
      ...authorIds
    );
    const grouped = new Map<number, any[]>();
    for (const row of rows) {
      const list = grouped.get(row.author_id) || [];
      list.push(row);
      grouped.set(row.author_id, list);
    }
    return authorIds.map((id) => grouped.get(id) || []);
  });
}

export const postResolvers = {
  /**
   * Top-level query: fetch recent posts with optional limit.
   */
  recentPosts(_: any, args: any, ctx: GqlContext) {
    const limit = args.limit || 20;
    return trackedQuery(
      ctx.db,
      "SELECT id, title, body, author_id, created_at FROM posts ORDER BY created_at DESC LIMIT ?",
      limit
    );
  },

  /**
   * Top-level query: posts by department.
   *
   * This intentionally uses a JOIN to eager-load user data alongside
   * posts, rather than using a DataLoader.  The JOIN is correct here
   * because we always need the user data for department filtering and
   * we are doing a single query — not an N+1 pattern.
   */
  postsByDepartment(_: any, args: { department: string; limit?: number }, ctx: GqlContext) {
    const limit = args.limit || 50;
    const rows = trackedQuery(
      ctx.db,
      `SELECT p.id, p.title, p.body, p.author_id, p.created_at,
              u.id AS user_id, u.name AS user_name, u.email AS user_email, u.department AS user_department
       FROM posts p
       JOIN users u ON u.id = p.author_id
       WHERE u.department = ?
       ORDER BY p.created_at DESC
       LIMIT ?`,
      args.department,
      limit
    );

    return rows.map((row: any) => ({
      id: row.id,
      title: row.title,
      body: row.body,
      author_id: row.author_id,
      created_at: row.created_at,
      author: {
        id: row.user_id,
        name: row.user_name,
        email: row.user_email,
        department: row.user_department,
      },
    }));
  },

  /**
   * Field resolver: Post.author — resolve the author of a post.
   *
   * Creates a new DataLoader for each Post being resolved, ensuring
   * clean loader state per resolution.
   */
  postAuthor(post: any, _args: any, ctx: GqlContext) {
    // If author was already eager-loaded (e.g. from postsByDepartment), return it
    if (post.author) {
      return post.author;
    }

    const loader = makePostsByAuthorLoader(ctx);
    // NOTE: this loader is for fetching a single user by ID.
    // We create a one-off loader each time to keep things isolated.
    const userLoader = new DataLoader(async (ids: readonly number[]) => {
      const placeholders = ids.map(() => "?").join(",");
      const rows = trackedQuery(
        ctx.db,
        `SELECT id, name, email, department FROM users WHERE id IN (${placeholders})`,
        ...ids
      );
      const byId = new Map(rows.map((r: any) => [r.id, r]));
      return ids.map((id) => byId.get(id) || null);
    });

    return userLoader.load(post.author_id);
  },
};
