#!/usr/bin/env node
"use strict";
const { spawnSync } = require("node:child_process");

let bin;
try {
  bin = require.resolve("@doskoi64/optioner-darwin-arm64/bin/optioner");
} catch {
  console.error(
    `optioner: no prebuilt binary for ${process.platform}-${process.arch} ` +
      "(v1 ships darwin-arm64 only)"
  );
  process.exit(1);
}

const result = spawnSync(bin, process.argv.slice(2), { stdio: "inherit" });
process.exit(result.status ?? 1);
