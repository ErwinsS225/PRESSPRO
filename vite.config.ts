import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import path from 'path';
import { defineConfig } from 'vite';

export default defineConfig(() => {
  return {
    plugins: [react(), tailwindcss()],
    resolve: {
      alias: {
        '@': path.resolve(import.meta.dirname, 'src'),
      },
    },
    build: {
      // Le paquet applicatif dépassait 600 kB : les dépendances stables
      // (React, icônes, animations) sont isolées dans des lots distincts, ce qui
      // permet au navigateur de les mettre en cache indépendamment du code
      // métier, et supprime l'avertissement « chunk > 500 kB ».
      rolldownOptions: {
        output: {
          codeSplitting: {
            groups: [
              { name: 'vendor-react', test: /node_modules\/(react|react-dom|scheduler)\// },
              { name: 'vendor-icons', test: /node_modules\/lucide-react\// },
              { name: 'vendor-motion', test: /node_modules\/(motion|framer-motion)\// },
              { name: 'vendor', test: /node_modules\// },
            ],
          },
        },
      },
    },
    server: {
      // HMR is disabled in AI Studio via DISABLE_HMR env var.
      // Do not modify—file watching is disabled to prevent flickering during agent edits.
      hmr: process.env.DISABLE_HMR !== 'true',
      // Disable file watching when DISABLE_HMR is true to save CPU during agent edits.
      watch: process.env.DISABLE_HMR === 'true' ? null : {},
    },
  };
});
