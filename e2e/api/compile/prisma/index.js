/**
 * Copyright 2025 The OPA Authors
 * SPDX-License-Identifier: Apache-2.0
 */

import util from "util";
import { PrismaClient } from "./generated/prisma/index.js";
import { ucastToPrisma } from "@open-policy-agent/ucast-prisma";
import { PrismaPg } from "@prisma/adapter-pg";
import pg from "pg";

const connectionString = process.env.DATABASE_URL || "postgresql://dummy:dummy@localhost:5432/dummy";
const pool = new pg.Pool({ connectionString });
const adapter = new PrismaPg(pool);
const prisma = new PrismaClient({ adapter });

// Function to read JSON from stdin
async function getStdinJson() {
  const chunks = [];
  for await (const chunk of process.stdin) {
    chunks.push(chunk);
  }
  const data = Buffer.concat(chunks).toString();
  try {
    return JSON.parse(data);
  } catch (error) {
    throw new Error("Invalid JSON input");
  }
}

async function main() {
  try {
    const filters = await getStdinJson();
    if (filters === null) return;

    const where = ucastToPrisma(filters, "fruit", {
      fruits: { $self: "fruit" },
    });
    console.error(util.inspect(where, { depth: null }));

    const results = await prisma.fruit.findMany({ where });
    process.stdout.write(JSON.stringify(results, null, 2));
  } catch (error) {
    console.error("Error:", error.message);
    process.exit(1);
  } finally {
    await prisma.$disconnect();
  }
}

// Run the script
main();
