import Database from "better-sqlite3";
import path from "path";
import fs from "fs";

const DB_PATH = process.env.DB_PATH || path.join(__dirname, "..", "data", "app.db");

const DEPARTMENTS = ["Engineering", "Marketing", "Sales", "Support", "Product"];

/**
 * Seed the database with 100 users and 500 posts (5 per user).
 * Each user is assigned to a department round-robin.
 */
function seed() {
  const dir = path.dirname(DB_PATH);
  if (!fs.existsSync(dir)) {
    fs.mkdirSync(dir, { recursive: true });
  }

  // Remove existing DB so we start fresh
  if (fs.existsSync(DB_PATH)) {
    fs.unlinkSync(DB_PATH);
  }

  const db = new Database(DB_PATH);
  db.pragma("journal_mode = WAL");

  db.exec(`
    CREATE TABLE IF NOT EXISTS users (
      id INTEGER PRIMARY KEY,
      name TEXT NOT NULL,
      email TEXT NOT NULL UNIQUE,
      department TEXT NOT NULL
    );

    CREATE TABLE IF NOT EXISTS posts (
      id INTEGER PRIMARY KEY,
      title TEXT NOT NULL,
      body TEXT NOT NULL,
      author_id INTEGER NOT NULL REFERENCES users(id),
      created_at TEXT NOT NULL DEFAULT (datetime('now'))
    );

    CREATE INDEX IF NOT EXISTS idx_posts_author_id ON posts(author_id);
    CREATE INDEX IF NOT EXISTS idx_users_department ON users(department);
  `);

  const insertUser = db.prepare(
    "INSERT INTO users (id, name, email, department) VALUES (?, ?, ?, ?)"
  );

  const insertPost = db.prepare(
    "INSERT INTO posts (id, title, body, author_id, created_at) VALUES (?, ?, ?, ?, ?)"
  );

  const insertUsers = db.transaction(() => {
    for (let i = 1; i <= 100; i++) {
      const dept = DEPARTMENTS[(i - 1) % DEPARTMENTS.length];
      insertUser.run(i, `User ${i}`, `user${i}@example.com`, dept);
    }
  });

  const insertPosts = db.transaction(() => {
    let postId = 1;
    for (let userId = 1; userId <= 100; userId++) {
      for (let j = 1; j <= 5; j++) {
        const date = new Date(2024, 0, 1 + postId).toISOString();
        insertPost.run(
          postId,
          `Post ${j} by User ${userId}`,
          `This is the body of post ${j} written by user ${userId}. It discusses various topics.`,
          userId,
          date
        );
        postId++;
      }
    }
  });

  insertUsers();
  insertPosts();

  const userCount = (db.prepare("SELECT COUNT(*) AS c FROM users").get() as any).c;
  const postCount = (db.prepare("SELECT COUNT(*) AS c FROM posts").get() as any).c;

  console.log(`Seeded database: ${userCount} users, ${postCount} posts`);

  db.close();
}

seed();
