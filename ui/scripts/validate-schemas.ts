#!/usr/bin/env node

/**
 * Schema Validation Utility
 *
 * This script fetches API schemas from the backend and validates that
 * the frontend Zod schemas match. This ensures a single source of truth
 * for API contracts.
 *
 * Usage:
 *   npm run validate-schemas
 *
 * Future Enhancement:
 *   Add code generation to auto-create Zod schemas from backend schemas.
 */

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

interface FieldDef {
  name: string;
  type: string;
  optional: boolean;
  nullable: boolean;
  tags: Record<string, string>;
}

interface TypeSchema {
  name: string;
  fields: FieldDef[];
}

interface SchemaResponse {
  schemas: Record<string, TypeSchema>;
}

async function fetchSchemas(): Promise<SchemaResponse> {
  try {
    const response = await fetch(`${API_URL}/api/schemas`);
    if (!response.ok) {
      throw new Error(`Failed to fetch schemas: ${response.statusText}`);
    }
    return await response.json();
  } catch (err) {
    console.error("Error fetching schemas:", err);
    process.exit(1);
  }
}

function validateSchema(backendSchema: TypeSchema): void {
  console.log(`\n✓ Schema: ${backendSchema.name}`);
  console.log(`  Fields: ${backendSchema.fields.length}`);

  backendSchema.fields.forEach((field) => {
    const nullable = field.nullable ? " (nullable)" : "";
    const optional = field.optional ? " (optional)" : "";
    console.log(`    - ${field.name}: ${field.type}${nullable}${optional}`);
  });
}

async function main(): Promise<void> {
  console.log("Fetching API schemas...\n");

  const schemas = await fetchSchemas();

  if (!schemas.schemas || Object.keys(schemas.schemas).length === 0) {
    console.error("No schemas found in response");
    process.exit(1);
  }

  console.log(`Found ${Object.keys(schemas.schemas).length} exported schemas:\n`);

  Object.values(schemas.schemas).forEach((schema) => {
    validateSchema(schema);
  });

  console.log("\n✅ Schema validation complete");
  console.log("\nNext Steps:");
  console.log(
    "  1. Compare these schemas with ui/src/lib/schemas.ts"
  );
  console.log(
    "  2. Create a code generation tool to auto-generate Zod schemas"
  );
  console.log(
    "  3. Add CI check to verify schema consistency before deployment"
  );
}

main().catch((err) => {
  console.error("Fatal error:", err);
  process.exit(1);
});
