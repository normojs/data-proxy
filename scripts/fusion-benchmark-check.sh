#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

BENCH="node tools/fusion-benchmark.mjs"

echo "[fusion-benchmark] self-test"
$BENCH self-test

echo "[fusion-benchmark] validate config"
$BENCH validate-config

echo "[fusion-benchmark] validate fresh eval datasets"
for dataset in \
  tools/fusion-benchmark/data/fresh-eval.example.jsonl \
  tools/fusion-benchmark/data/fresh-eval.v1-differentiated.jsonl \
  tools/fusion-benchmark/data/fresh-eval.v1-pilot.jsonl \
  tools/fusion-benchmark/data/fresh-eval.v2-pilot.jsonl
do
  $BENCH fresh-validate --dataset "$dataset"
done

echo "[fusion-benchmark] validate code eval datasets"
for dataset in \
  tools/fusion-benchmark/data/code-eval.example.jsonl \
  tools/fusion-benchmark/data/code-eval.v1.jsonl
do
  $BENCH code-validate --dataset "$dataset"
done

echo "[fusion-benchmark] check candidate files for committed secrets"
if rg -n --hidden --glob '!tools/fusion-benchmark/.env.local' --glob '!tools/fusion-benchmark/runs/**' --glob '!tools/fusion-benchmark/reports/**' \
  'sk-[A-Za-z0-9]{20,}|AKIA[0-9A-Z]{16}|AIza[0-9A-Za-z_-]{35}|xox[baprs]-[0-9A-Za-z-]+|gh[pousr]_[A-Za-z0-9_]{20,}|eyJ[A-Za-z0-9_-]{20,}' \
  tools/fusion-benchmark tools/fusion-benchmark.mjs
then
  echo "[fusion-benchmark] possible committed secret found" >&2
  exit 1
fi

echo "[fusion-benchmark] ok"
