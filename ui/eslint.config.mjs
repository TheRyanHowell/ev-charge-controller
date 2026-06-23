import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTs from "eslint-config-next/typescript";
import prettierRecommended from "eslint-plugin-prettier/recommended";
import ts from "typescript-eslint";
import importPlugin from "eslint-plugin-import";
import unicorn from "eslint-plugin-unicorn";
import perfectionist from "eslint-plugin-perfectionist";
import reactRefresh from "eslint-plugin-react-refresh";

const eslintConfig = defineConfig([
  {
    linterOptions: {
      noInlineConfig: true,
    },
  },
  ...nextVitals,
  ...nextTs,
  globalIgnores([
    ".next/**",
    "out/**",
    "build/**",
    "next-env.d.ts",
  ]),
  prettierRecommended,

  // TypeScript strict rules (without type-aware rules that need project context)
  ts.configs["strict"],

  // Import ordering and validation
  importPlugin.configs["recommended", "typescript"],
  {
    plugins: {
      import: importPlugin,
    },
    settings: {
      "import/parsers": {
        "@typescript-eslint/parser": [".ts", ".tsx"],
      },
      "import/resolver": {
        typescript: {
          project: "./tsconfig.json",
        },
      },
    },
    rules: {
      "import/no-unresolved": "off", // Next.js app router generates dynamic routes
      "import/no-relative-packages": "error",
    },
  },

  // Perfectionist: sort imports, exports, objects
  {
    plugins: {
      perfectionist: perfectionist,
    },
    rules: {
      "perfectionist/sort-array-includes": "error",
      "perfectionist/sort-imports": [
        "error",
        {
          internalPattern: ["@/*", "./*", "../*"],
        },
      ],
      "perfectionist/sort-exports": "error",
      "perfectionist/sort-named-exports": "error",
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

  // Fast Refresh validation
  reactRefresh.configs.recommended,

  // Additional rules on top of all presets
  {
    rules: {
      "@typescript-eslint/no-unused-vars": [
        "error",
        { argsIgnorePattern: "^_", varsIgnorePattern: "^_" },
      ],
      "@typescript-eslint/no-explicit-any": "error",
      "@typescript-eslint/no-non-null-assertion": "error",
      "react-hooks/exhaustive-deps": "error",
      "react-hooks/set-state-in-effect": "warn",
      "no-unsafe-optional-chaining": "error",
      "no-constant-condition": ["error", { checkLoops: false }],
      "react-refresh/only-export-components": [
        "warn",
        {
          allowConstantExport: true,
        },
      ],
    },
  },

  // Test utilities are exempt from fast-refresh rules
  {
    files: ["**/test-utils.{ts,tsx}", "**/*.test.{ts,tsx}", "**/*.spec.{ts,tsx}"],
    rules: {
      "react-refresh/only-export-components": "off",
    },
  },

  // Next.js layout/page files export metadata (and other route config) alongside
  // components - exempt from fast-refresh rules
  {
    files: ["**/layout.tsx", "**/page.tsx"],
    rules: {
      "react-refresh/only-export-components": "off",
    },
  },

  // useChargeSession uses setState-in-effect to sync React Query cache with local state
  // This is an intentional synchronization pattern between external store (query cache) and React state
  {
    files: ["**/hooks/useChargeSession.ts"],
    rules: {
      "react-hooks/set-state-in-effect": "off",
    },
  },

  // useSessionPolling seeds initial session data into query cache on mount (SSR hydration)
  // Empty dep array is intentional - only runs once on mount
  {
    files: ["**/hooks/useSessionPolling.ts"],
    rules: {
      "react-hooks/exhaustive-deps": "warn",
    },
  },

  // FirstRunWizard syncs plug online status from React Query to local state
  // SettingsModal syncs modal form state when opened (prop-to-state sync)
  // SpeedometerGauge clears local drag state when props catch up from API
  // usePlug resets selectedPlugId when initialPlugs change (prop-to-state sync)
  // All are valid external-system synchronization patterns
  {
    files: [
      "**/FirstRunWizard.tsx",
      "**/SettingsModal.tsx",
      "**/SpeedometerGauge.tsx",
      "**/hooks/usePlug.ts",
    ],
    rules: {
      "react-hooks/set-state-in-effect": "off",
    },
  },

  // Next.js page files export metadata alongside components
  // VehicleSelector re-exports parseVehicleSelectorValue utility
  {
    files: [
      "**/login/page.tsx",
      "**/VehicleSelector.tsx",
    ],
    rules: {
      "react-refresh/only-export-components": "off",
    },
  },

  // Test file adjustments - vitest mock reconfiguration requires as any
  // MUST be placed after all other rules to take precedence
  {
    files: ["**/*.test.{ts,tsx}", "**/*.spec.{ts,tsx}"],
    rules: {
      "@typescript-eslint/no-explicit-any": "off",
    },
  },
]);

export default eslintConfig;
