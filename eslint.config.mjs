import js from '@eslint/js';
import typescriptEslint from 'typescript-eslint';
import importPlugin from 'eslint-plugin-import';

export default [
  { ignores: ['dist/', 'node_modules/', 'src-tauri/target/', 'src/vite-env.d.ts'] },
  {
    files: ['**/*.ts', '**/*.tsx'],
    languageOptions: {
      ecmaVersion: 2020,
      sourceType: 'module',
      parserOptions: { project: './tsconfig.json' },
    },
    settings: { 'import/resolver': { typescript: { project: './tsconfig.json' } } },
    plugins: { '@typescript-eslint': typescriptEslint.plugin, import: importPlugin },
    rules: {
      ...typescriptEslint.configs.recommended.rules,
      'no-unused-vars': 'off',
      '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_' }],
    },
  },
  {
    files: ['**/*.js', '**/*.cjs'],
    languageOptions: { ecmaVersion: 2020, sourceType: 'commonjs' },
    rules: { 'no-unused-vars': ['error', { argsIgnorePattern: '^_' }] },
  },
  {
    files: ['vite.config.ts'],
    rules: { 'import/order': 'off' },
  },
];
