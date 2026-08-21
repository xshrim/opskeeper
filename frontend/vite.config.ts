import { svelte } from '@sveltejs/vite-plugin-svelte';
import { defineConfig } from 'vite';

const basePathPattern =
  /^\/(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)(?:\/[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)*$/;

export default defineConfig(({ command }) => {
  const basePath = process.env.OPSK_BASE_PATH ?? '/opskeeper';
  if (
    basePath !== '/' &&
    (basePath.length > 128 || !basePathPattern.test(basePath))
  ) {
    throw new Error(
      'OPSK_BASE_PATH must be / or a slash-prefixed path of lowercase letters, digits, or internal hyphens'
    );
  }

  const baseHref = basePath === '/' ? '/' : `${basePath}/`;
  const route = (suffix: string) =>
    basePath === '/' ? suffix : `${basePath}${suffix}`;
  return {
    base: command === 'serve' ? baseHref : './',
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
            `href="${baseHref}" data-opsk-runtime-base`
          );
        }
      }
    ],
    server: {
      port: 5173,
      strictPort: true,
      proxy: {
        [route('/api')]: 'http://localhost:8080',
        [route('/health')]: 'http://localhost:8080'
      }
    },
    build: {
      // The complete Lucide registry powers the searchable team icon picker.
      // It is isolated and cacheable, so use a threshold above that intentional chunk.
      chunkSizeWarningLimit: 800,
      rollupOptions: {
        output: {
          manualChunks(id) {
            if (!id.includes('/node_modules/')) return undefined;
            if (id.includes('/node_modules/svelte/')) return 'vendor-svelte';
            if (id.includes('/node_modules/lucide-svelte/')) return 'vendor-icons';
            if (id.includes('/node_modules/simple-icons/')) return 'vendor-brands';
            return 'vendor';
          }
        }
      }
    },
    test: {
      environment: 'jsdom',
      include: ['src/**/*.test.ts']
    }
  };
});
