#!/bin/bash
# Usage: ./runner.sh <input_file> <golden_file>

INPUT=$1
GOLDEN=$2
TMP_OUT="tmp_output.json"

# Time execution and capture exit code
start_time=$(date +%s%N)
./main-binary -input "$INPUT" > "$TMP_OUT"
end_time=$(date +%s%N)

# Checksum Comparison
if diff "$TMP_OUT" "$GOLDEN" > /dev/null; then
    status="PASS"
else
    status="FAIL"
fi

# Calculate duration in milliseconds
duration=$(( (end_time - start_time) / 1000000 ))
echo "CLI_TASK,$duration,$status" >> benchmarks/raw_results.csv
