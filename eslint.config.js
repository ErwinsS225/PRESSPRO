// Configuration ESLint (flat config, ESLint 9) — PressingOS.
//
// Objectifs : détecter les erreurs réelles (variables inutilisées, hooks React
// mal employés, promesses flottantes) sans imposer de style : le formatage est
// délégué à Prettier (eslint-config-prettier désactive les règles de style).
import js from '@eslint/js';
import globals from 'globals';
import reactHooks from 'eslint-plugin-react-hooks';
import reactRefresh from 'eslint-plugin-react-refresh';
import tseslint from 'typescript-eslint';
import prettier from 'eslint-config-prettier';

export default tseslint.config(
  {
    ignores: ['dist/**', 'node_modules/**', 'coverage/**', 'backend/**'],
  },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  {
    files: ['**/*.{ts,tsx}'],
    languageOptions: {
      ecmaVersion: 2022,
      globals: { ...globals.browser, ...globals.node },
    },
    plugins: {
      'react-hooks': reactHooks,
      'react-refresh': reactRefresh,
    },
    rules: {
      ...reactHooks.configs.recommended.rules,
      // Règles issues du compilateur React (plugin v7) : utiles comme signaux,
      // mais le chargement de données via useEffect et les calculs d'affichage
      // (URL de QR code, dates) sont des usages assumés ici — avertissements
      // plutôt que blocages, pour ne pas bloquer la CI sur du style.
      'react-hooks/set-state-in-effect': 'warn',
      'react-hooks/purity': 'warn',
      'react-refresh/only-export-components': ['warn', { allowConstantExport: true }],
      '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_', varsIgnorePattern: '^_' }],
      '@typescript-eslint/no-explicit-any': 'warn',
      'no-console': ['warn', { allow: ['warn', 'error'] }],
      eqeqeq: ['error', 'smart'],
      'prefer-const': 'error',
    },
  },
  {
    files: ['**/*.test.{ts,tsx}', 'src/test/**'],
    rules: {
      'no-console': 'off',
    },
  },
  {
    // Entrée serveur (Express + proxy Vite) : les traces console sont le canal
    // de journalisation du processus.
    files: ['server.ts'],
    rules: {
      'no-console': 'off',
    },
  },
  prettier
);
