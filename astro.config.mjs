import { defineConfig } from 'astro/config';
import sitemap from '@astrojs/sitemap';

export default defineConfig({
  site: 'https://needle-bench.cc',
  output: 'static',
  trailingSlash: 'always',
  redirects: {
    '/leaderboard': '/',
  },
  integrations: [
    sitemap({
      filter: (page) => !page.includes('/leaderboard'),
    }),
  ],
  build: {
    format: 'directory',
  },
});
