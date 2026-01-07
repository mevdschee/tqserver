#!/bin/bash
# Demo script for Kotlin API Worker
# This script demonstrates the CRUD operations

set -e

BASE_URL="${BASE_URL:-http://localhost:3000/api}"

echo "=========================================="
echo "TQServer Kotlin API Worker Demo"
echo "=========================================="
echo "Base URL: $BASE_URL"
echo ""

# Check if worker is running
echo "1. Health Check"
echo "   GET $BASE_URL/health"
if curl -s -f "$BASE_URL/health" > /dev/null 2>&1; then
    echo "   ✓ Worker is healthy"
else
    echo "   ✗ Worker is not responding"
    echo "   Make sure TQServer is running with the API worker"
    exit 1
fi
echo ""

# Get service info
echo "2. Service Information"
echo "   GET $BASE_URL/"
curl -s "$BASE_URL/" | jq '.'
echo ""

# Create items
echo "3. Create Items"
echo "   POST $BASE_URL/items"
ITEM1=$(curl -s -X POST "$BASE_URL/items" \
  -H "Content-Type: application/json" \
  -d '{"name":"Laptop","description":"Dell XPS 15"}')
echo "$ITEM1" | jq '.'
ITEM1_ID=$(echo "$ITEM1" | jq -r '.id')

ITEM2=$(curl -s -X POST "$BASE_URL/items" \
  -H "Content-Type: application/json" \
  -d '{"name":"Mouse","description":"Logitech MX Master"}')
echo "$ITEM2" | jq '.'
ITEM2_ID=$(echo "$ITEM2" | jq -r '.id')

ITEM3=$(curl -s -X POST "$BASE_URL/items" \
  -H "Content-Type: application/json" \
  -d '{"name":"Keyboard","description":"Mechanical keyboard"}')
echo "$ITEM3" | jq '.'
echo ""

# List all items
echo "4. List All Items"
echo "   GET $BASE_URL/items"
curl -s "$BASE_URL/items" | jq '.'
echo ""

# Get single item
echo "5. Get Single Item"
echo "   GET $BASE_URL/items/$ITEM1_ID"
curl -s "$BASE_URL/items/$ITEM1_ID" | jq '.'
echo ""

# Update item
echo "6. Update Item"
echo "   PUT $BASE_URL/items/$ITEM2_ID"
curl -s -X PUT "$BASE_URL/items/$ITEM2_ID" \
  -H "Content-Type: application/json" \
  -d '{"name":"Wireless Mouse","description":"Logitech MX Master 3"}' | jq '.'
echo ""

# Get stats
echo "7. Get Statistics"
echo "   GET $BASE_URL/stats"
curl -s "$BASE_URL/stats" | jq '.'
echo ""

# Delete item
echo "8. Delete Item"
echo "   DELETE $BASE_URL/items/$ITEM1_ID"
curl -s -X DELETE "$BASE_URL/items/$ITEM1_ID" | jq '.'
echo ""

# List remaining items
echo "9. List Remaining Items"
echo "   GET $BASE_URL/items"
curl -s "$BASE_URL/items" | jq '.'
echo ""

# Test error handling
echo "10. Error Handling - Get Non-existent Item"
echo "    GET $BASE_URL/items/999"
curl -s "$BASE_URL/items/999" | jq '.'
echo ""

echo "11. Error Handling - Create Item with Empty Name"
echo "    POST $BASE_URL/items"
curl -s -X POST "$BASE_URL/items" \
  -H "Content-Type: application/json" \
  -d '{"name":"","description":"Test"}' | jq '.'
echo ""

echo "=========================================="
echo "Demo Complete!"
echo "=========================================="
