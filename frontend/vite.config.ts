import { svelte } from '@sveltejs/vite-plugin-svelte';
import { defineConfig } from 'vite';

const prefixPattern = /^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$/;

export default defineConfig(({ command }) => {
  const prefix = process.env.OPSK_PREFIX ?? 'opskeeper';
  if (prefix.length > 40 || !prefixPattern.test(prefix)) {
    throw new Error(
      'OPSK_PREFIX must contain 1-40 lowercase letters, digits, or internal hyphens'
    );
  }

  const basePath = `/${prefix}`;
  return {
    base: command === 'serve' ? `${basePath}/` : './',
    plugins: [
      svelte(),
      {
        name: 'opskeeper-development-base',
        transformIndexHtml(html) {
          if (command !== 'serve') {
            return html;
          }
          return html.replace(
            'href="./" data-opsk-runtime-base',
            `href="${basePath}/" data-opsk-runtime-base`
          );
        }
      }
    ],
    server: {
      port: 5173,
      strictPort: true,
      proxy: {
        [`${basePath}/api`]: 'http://localhost:8080',
        [`${basePath}/health`]: 'http://localhost:8080'
      }
    },
    test: {
      environment: 'jsdom',
      include: ['src/**/*.test.ts']
    }
  };
});
