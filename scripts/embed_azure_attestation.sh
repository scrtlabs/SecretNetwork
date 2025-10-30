#!/bin/bash
set -e  # exit immediately if any command fails
set -u  # treat unset variables as errors
set -euo pipefail

if [ $# -lt 1 ] || [ $# -gt 2 ]; then
    echo "Usage: $0 <source_file> [output_file]"
    exit 1
fi

SOURCE_FILE="$1"
OUTPUT_FILE="${2:-$SOURCE_FILE}"  # If no second arg, overwrite source

echo "Parsing source file " $SOURCE_FILE " ..."

read n1 n2 n3 < <(od -An -t u4 -N 12 -L $SOURCE_FILE)

header_size=12
block2_b64=$(dd if=$SOURCE_FILE bs=1 skip=$((header_size+n1)) count=$n2 2>/dev/null | base64 -w0)
block3_b64=$(dd if=$SOURCE_FILE bs=1 skip=$((header_size+n1+n2)) count=$n3 2>/dev/null | base64 -w0)

echo "Obtaining azure attestation ..."

response=$(curl -s -X POST https://sharedeus2.eus2.attest.azure.net/attest/SgxEnclave?api-version=2020-10-01 \
     -H "Content-Type: application/json" \
     -d '{"Quote": "'"$block2_b64"'"}')

token=$(echo "$response" | jq -r '.token')
#echo "Token: $token"

# Function to append KV
append_kv() {
    local key=$1
    local file=$2
    local size
    size=$(stat -c%s "$file")  # get file size in bytes

    # Write key (1 byte)
    printf "\\x$(printf '%02x' "$key")" >> "$OUTPUT_FILE"

    # Write size as u32 LE
    printf "%02x%02x%02x%02x" $((size & 0xFF)) $(( (size >> 8) & 0xFF )) \
                              $(( (size >> 16) & 0xFF )) $(( (size >> 24) & 0xFF )) \
        | xxd -r -p >> "$OUTPUT_FILE"

    # Append the value
    cat "$file" >> "$OUTPUT_FILE"
}

echo "Creating attestation file " $OUTPUT_FILE " ..."

# Output file
> "$OUTPUT_FILE"  # empty the file

tmpdir=$(mktemp -d)

echo "$block2_b64" | base64 -d > "$tmpdir/block2.bin"
echo "$block3_b64" | base64 -d > "$tmpdir/block3.bin"
echo -n "$token" > "$tmpdir/token"


append_kv 2 "$tmpdir/block2.bin"
append_kv 3 "$tmpdir/block3.bin"
append_kv 4 "$tmpdir/token"

rm -rf "$tmpdir"

echo "Done"
