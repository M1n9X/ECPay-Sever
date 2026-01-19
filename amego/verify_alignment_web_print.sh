#!/bin/bash
# Verification script to compare Python and TypeScript alignment for web_print

echo "=========================================="
echo "WEB_PRINT ALIGNMENT VERIFICATION"
echo "=========================================="
echo ""

# Run Python test and extract key values
echo "Running Python test..."
PY_OUTPUT=$(python3 test_alignment_web_print.py 2>&1)
PY_LENGTH=$(echo "$PY_OUTPUT" | grep "Length:" | head -1 | awk '{print $NF}')
PY_SIG=$(echo "$PY_OUTPUT" | grep "MD5 Signature:" | head -1 | awk '{print $NF}')

echo "Python Results:"
echo "  JSON Length: $PY_LENGTH"
echo "  MD5 Signature: $PY_SIG"
echo ""

# Run TypeScript test and extract key values
echo "Running TypeScript test..."
TS_OUTPUT=$(npx tsx test_alignment_web_print.ts 2>&1)
TS_LENGTH=$(echo "$TS_OUTPUT" | grep "JSON Length:" | tail -1 | awk '{print $NF}')
TS_SIG=$(echo "$TS_OUTPUT" | grep "MD5 Signature:" | head -1 | awk '{print $NF}')

echo "TypeScript Results:"
echo "  JSON Length: $TS_LENGTH"
echo "  MD5 Signature: $TS_SIG"
echo ""

# Compare
echo "=========================================="
echo "COMPARISON RESULT"
echo "=========================================="

if [ "$PY_LENGTH" = "$TS_LENGTH" ] && [ "$PY_SIG" = "$TS_SIG" ]; then
    echo "✅ SUCCESS! Python and TypeScript outputs are IDENTICAL"
    echo ""
    echo "Both produce:"
    echo "  JSON Length: $PY_LENGTH"
    echo "  MD5 Signature: $PY_SIG"
    exit 0
else
    echo "❌ MISMATCH DETECTED!"
    echo ""
    if [ "$PY_LENGTH" != "$TS_LENGTH" ]; then
        echo "  JSON Length mismatch:"
        echo "    Python:     $PY_LENGTH"
        echo "    TypeScript: $TS_LENGTH"
    fi
    if [ "$PY_SIG" != "$TS_SIG" ]; then
        echo "  MD5 Signature mismatch:"
        echo "    Python:     $PY_SIG"
        echo "    TypeScript: $TS_SIG"
    fi
    exit 1
fi
