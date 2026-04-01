import express from "express";
import { graphqlHTTP } from "express-graphql";
import { schema } from "./schema";
import { createContext } from "./context";
import { resetQueryCount, getQueryCount } from "./db";

const app = express();
const PORT = process.env.PORT || 4000;

/**
 * Main GraphQL endpoint.
 *
 * The context factory runs once per request, providing each resolver
 * tree with a shared database connection.
 *
 * Query counting: the X-Reset-Query-Count header resets the counter
 * before the query executes, and the response includes the count in
 * X-Query-Count.  This lets the performance test measure exactly how
 * many DB queries a single GraphQL request triggers.
 */
app.use(
  "/graphql",
  (req, res, next) => {
    if (req.headers["x-reset-query-count"]) {
      resetQueryCount();
    }
    next();
  },
  graphqlHTTP(() => ({
    schema,
    context: createContext(),
    graphiql: false,
    customFormatErrorFn: (err) => ({
      message: err.message,
      locations: err.locations,
      path: err.path,
    }),
    extensions: () => ({
      queryCount: getQueryCount(),
    }),
  }))
);

/**
 * Health check endpoint.
 */
app.get("/health", (_req, res) => {
  res.json({ status: "ok" });
});

/**
 * Diagnostics endpoint — returns current query count without resetting.
 */
app.get("/query-count", (_req, res) => {
  res.json({ count: getQueryCount() });
});

app.listen(PORT, () => {
  console.log(`GraphQL server running on http://localhost:${PORT}/graphql`);
});

export default app;
