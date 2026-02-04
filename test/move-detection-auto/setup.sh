#!/usr/bin/env bash
# Setup script for move detection demo - AUTO-MERGE showcase
# This creates a scenario where move detection enables full auto-merge

set -e

DEMO_DIR="$(dirname "$0")/repo"

# Clean up any existing demo
rm -rf "$DEMO_DIR"
mkdir -p "$DEMO_DIR"
cd "$DEMO_DIR"

echo "=== Setting up auto-merge move detection demo ==="
echo ""

# Initialize git repo
git init
git config user.email "demo@example.com"
git config user.name "Demo User"

# Create initial Python file with several functions
cat > shopping.py << 'EOF'
"""Shopping cart utilities."""

def calc(items):
    """Calculate total price of items with tax."""
    subtotal = sum(item['price'] * item['quantity'] for item in items)
    tax = subtotal * 0.08
    shipping = 5.99 if subtotal < 50 else 0
    return round(subtotal + tax + shipping, 2)


def fmt_price(amount):
    """Format amount as USD price string."""
    if amount < 0:
        return f"-${abs(amount):.2f}"
    return f"${amount:.2f}"


def validate(item):
    """Validate an item has required fields."""
    required = ['name', 'price', 'quantity']
    for field in required:
        if field not in item:
            raise ValueError(f"Missing field: {field}")
    if item['price'] < 0:
        raise ValueError("Price cannot be negative")
    if item['quantity'] < 1:
        raise ValueError("Quantity must be at least 1")
    return True


def get_discount(customer_type, subtotal):
    """Calculate discount based on customer type."""
    if customer_type == 'premium':
        return subtotal * 0.15
    elif customer_type == 'member':
        return subtotal * 0.10
    return 0
EOF

# Create initial JavaScript file
cat > api.js << 'EOF'
/**
 * API utilities
 */

function fetchData(endpoint) {
    return fetch(`/api/${endpoint}`)
        .then(response => {
            if (!response.ok) {
                throw new Error(`HTTP error: ${response.status}`);
            }
            return response.json();
        })
        .then(data => {
            return {
                success: true,
                data: data,
                timestamp: new Date().toISOString()
            };
        });
}

function sendData(endpoint, payload) {
    return fetch(`/api/${endpoint}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
    })
        .then(response => response.json());
}

function formatDate(date) {
    const options = { year: 'numeric', month: 'long', day: 'numeric' };
    return date.toLocaleDateString('en-US', options);
}
EOF

git add .
git commit -m "Initial commit with shopping.py and api.js"

echo "Created initial commit"

# Create feature branch - REFACTORING: rename functions to clearer names
git checkout -b feature/refactor-naming

cat > shopping.py << 'EOF'
"""Shopping cart utilities."""

def calculate_cart_total(items):
    """Calculate total price of items with tax."""
    subtotal = sum(item['price'] * item['quantity'] for item in items)
    tax = subtotal * 0.08
    shipping = 5.99 if subtotal < 50 else 0
    return round(subtotal + tax + shipping, 2)


def format_currency(amount):
    """Format amount as USD price string."""
    if amount < 0:
        return f"-${abs(amount):.2f}"
    return f"${amount:.2f}"


def validate_cart_item(item):
    """Validate an item has required fields."""
    required = ['name', 'price', 'quantity']
    for field in required:
        if field not in item:
            raise ValueError(f"Missing field: {field}")
    if item['price'] < 0:
        raise ValueError("Price cannot be negative")
    if item['quantity'] < 1:
        raise ValueError("Quantity must be at least 1")
    return True


def calculate_loyalty_discount(customer_type, subtotal):
    """Calculate discount based on customer type."""
    if customer_type == 'premium':
        return subtotal * 0.15
    elif customer_type == 'member':
        return subtotal * 0.10
    return 0
EOF

cat > api.js << 'EOF'
/**
 * API utilities
 */

function fetchFromApi(endpoint) {
    return fetch(`/api/${endpoint}`)
        .then(response => {
            if (!response.ok) {
                throw new Error(`HTTP error: ${response.status}`);
            }
            return response.json();
        })
        .then(data => {
            return {
                success: true,
                data: data,
                timestamp: new Date().toISOString()
            };
        });
}

function postToApi(endpoint, payload) {
    return fetch(`/api/${endpoint}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
    })
        .then(response => response.json());
}

function formatDateLong(date) {
    const options = { year: 'numeric', month: 'long', day: 'numeric' };
    return date.toLocaleDateString('en-US', options);
}
EOF

git add .
git commit -m "Refactor: rename functions for clarity

- calc -> calculate_cart_total
- fmt_price -> format_currency
- validate -> validate_cart_item
- get_discount -> calculate_loyalty_discount
- fetchData -> fetchFromApi
- sendData -> postToApi
- formatDate -> formatDateLong"

echo "Created feature branch with refactored names"

# Go back to main - INDEPENDENTLY do the same refactoring (simulating parallel work)
git checkout main

# Main branch does the SAME renames (developers agreed on naming convention)
cat > shopping.py << 'EOF'
"""Shopping cart utilities."""

def calculate_cart_total(items):
    """Calculate total price of items with tax."""
    subtotal = sum(item['price'] * item['quantity'] for item in items)
    tax = subtotal * 0.08
    shipping = 5.99 if subtotal < 50 else 0
    return round(subtotal + tax + shipping, 2)


def format_currency(amount):
    """Format amount as USD price string."""
    if amount < 0:
        return f"-${abs(amount):.2f}"
    return f"${amount:.2f}"


def validate_cart_item(item):
    """Validate an item has required fields."""
    required = ['name', 'price', 'quantity']
    for field in required:
        if field not in item:
            raise ValueError(f"Missing field: {field}")
    if item['price'] < 0:
        raise ValueError("Price cannot be negative")
    if item['quantity'] < 1:
        raise ValueError("Quantity must be at least 1")
    return True


def calculate_loyalty_discount(customer_type, subtotal):
    """Calculate discount based on customer type."""
    if customer_type == 'premium':
        return subtotal * 0.15
    elif customer_type == 'member':
        return subtotal * 0.10
    return 0
EOF

cat > api.js << 'EOF'
/**
 * API utilities
 */

function fetchFromApi(endpoint) {
    return fetch(`/api/${endpoint}`)
        .then(response => {
            if (!response.ok) {
                throw new Error(`HTTP error: ${response.status}`);
            }
            return response.json();
        })
        .then(data => {
            return {
                success: true,
                data: data,
                timestamp: new Date().toISOString()
            };
        });
}

function postToApi(endpoint, payload) {
    return fetch(`/api/${endpoint}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
    })
        .then(response => response.json());
}

function formatDateLong(date) {
    const options = { year: 'numeric', month: 'long', day: 'numeric' };
    return date.toLocaleDateString('en-US', options);
}
EOF

git add .
git commit -m "Refactor: apply new naming convention"

echo "Created main branch with same refactoring"

echo ""
echo "==========================================="
echo "Demo repository created at: $DEMO_DIR"
echo ""
echo "Scenario: Two developers independently refactored"
echo "the same functions with identical new names."
echo ""
echo "Old names -> New names:"
echo "  calc           -> calculate_cart_total"
echo "  fmt_price      -> format_currency"
echo "  validate       -> validate_cart_item"
echo "  get_discount   -> calculate_loyalty_discount"
echo "  fetchData      -> fetchFromApi"
echo "  sendData       -> postToApi"
echo "  formatDate     -> formatDateLong"
echo ""
echo "To test:"
echo "  cd $DEMO_DIR"
echo "  g2 merge feature/refactor-naming"
echo "==========================================="
