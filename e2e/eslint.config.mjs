import { defineConfig, globalIgnores } from "eslint/config";
import prettierRecommended from "eslint-plugin-prettier/recommended";
import ts from "typescript-eslint";
import unicorn from "eslint-plugin-unicorn";

export default defineConfig([
  {
    linterOptions: {
      noInlineConfig: true,
    },
  },
  globalIgnores(["storage-states/"]),
  prettierRecommended,

  // TypeScript strict type-checked rules (required for no-floating-promises)
  ...ts.configs.strictTypeChecked,
  ...ts.configs.stylisticTypeChecked,
  {
    languageOptions: {
      parserOptions: {
        projectService: {
          allowDefaultProject: ["*.config.*"],
        },
        tsconfigRootDir: import.meta.dirname,
      },
    },
  },

  // Unicorn: modern JS patterns
  {
    plugins: {
      unicorn,
    },
    rules: {
      "unicorn/prefer-top-level-await": "error",
      "unicorn/prefer-module": "error",
      "unicorn/no-useless-promise-resolve-reject": "error",
      "unicorn/no-array-reduce": "off",
      "unicorn/no-nested-ternary": "off",
    },
  },

  // Additional rules
  {
    rules: {
      // Playwright docs recommend this to catch missing awaits on async API calls
      // https://playwright.dev/docs/test-configuration#linting
      "@typescript-eslint/no-floating-promises": "error",
      "@typescript-eslint/no-explicit-any": "error",
      "@typescript-eslint/no-non-null-assertion": "error",
      "@typescript-eslint/no-unused-vars": [
        "error",
        { argsIgnorePattern: "^_", varsIgnorePattern: "^_" },
      ],
      "no-unsafe-optional-chaining": "error",
      "no-constant-condition": ["error", { checkLoops: false }],
    },
  },
]);
