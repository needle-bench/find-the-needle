import {
  GraphQLObjectType,
  GraphQLSchema,
  GraphQLString,
  GraphQLInt,
  GraphQLList,
  GraphQLNonNull,
} from "graphql";
import { userResolvers } from "./resolvers/userResolver";
import { postResolvers } from "./resolvers/postResolver";

/**
 * User type — represents a user in the system.
 * The `posts` field triggers a DataLoader fetch for the user's posts.
 */
const UserType: GraphQLObjectType = new GraphQLObjectType({
  name: "User",
  fields: () => ({
    id: { type: new GraphQLNonNull(GraphQLInt) },
    name: { type: new GraphQLNonNull(GraphQLString) },
    email: { type: new GraphQLNonNull(GraphQLString) },
    department: { type: new GraphQLNonNull(GraphQLString) },
    posts: {
      type: new GraphQLList(PostType),
      resolve: userResolvers.userPosts,
    },
  }),
});

/**
 * Post type — represents a blog post.
 * The `author` field triggers a DataLoader fetch for the post's author.
 */
const PostType: GraphQLObjectType = new GraphQLObjectType({
  name: "Post",
  fields: () => ({
    id: { type: new GraphQLNonNull(GraphQLInt) },
    title: { type: new GraphQLNonNull(GraphQLString) },
    body: { type: new GraphQLNonNull(GraphQLString) },
    author_id: { type: new GraphQLNonNull(GraphQLInt) },
    created_at: { type: new GraphQLNonNull(GraphQLString) },
    author: {
      type: UserType,
      resolve: postResolvers.postAuthor,
    },
  }),
});

/**
 * Root query type.
 */
const QueryType = new GraphQLObjectType({
  name: "Query",
  fields: {
    users: {
      type: new GraphQLList(UserType),
      args: {
        limit: { type: GraphQLInt },
      },
      resolve: userResolvers.users,
    },
    user: {
      type: UserType,
      args: {
        id: { type: new GraphQLNonNull(GraphQLInt) },
      },
      resolve: userResolvers.user,
    },
    recentPosts: {
      type: new GraphQLList(PostType),
      args: {
        limit: { type: GraphQLInt },
      },
      resolve: postResolvers.recentPosts,
    },
    postsByDepartment: {
      type: new GraphQLList(PostType),
      args: {
        department: { type: new GraphQLNonNull(GraphQLString) },
        limit: { type: GraphQLInt },
      },
      resolve: postResolvers.postsByDepartment,
    },
  },
});

export const schema = new GraphQLSchema({
  query: QueryType,
});
