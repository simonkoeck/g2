#!/usr/bin/env bash
# Move Detection Showcase - Full Auto-Merge Demo
#
# Scenario: Main branch cleans up "deprecated" functions while
# feature branch renames them. Move detection recognizes they're
# the same code and auto-merges everything.

set -e

DEMO_DIR="$(dirname "$0")/repo"

rm -rf "$DEMO_DIR"
mkdir -p "$DEMO_DIR"
cd "$DEMO_DIR"

echo "=== Setting up Move Detection Showcase ==="
echo ""

git init
git config user.email "demo@example.com"
git config user.name "Demo User"

# Create initial files with "legacy" function names
cat > cart.py << 'EOF'
"""Shopping cart module."""

def calc_total(items):
    """Calculate the total price including tax and shipping."""
    subtotal = sum(item['price'] * item['quantity'] for item in items)
    tax_rate = 0.08
    tax = subtotal * tax_rate
    shipping = 5.99 if subtotal < 50 else 0
    total = subtotal + tax + shipping
    return round(total, 2)


def fmt_money(amount):
    """Format a number as a currency string."""
    if amount < 0:
        return f"-${abs(amount):,.2f}"
    return f"${amount:,.2f}"


def chk_item(item):
    """Check if a cart item is valid."""
    required_fields = ['name', 'price', 'quantity']
    for field in required_fields:
        if field not in item:
            raise ValueError(f"Missing required field: {field}")
    if not isinstance(item['price'], (int, float)):
        raise TypeError("Price must be a number")
    if item['price'] < 0:
        raise ValueError("Price cannot be negative")
    if not isinstance(item['quantity'], int):
        raise TypeError("Quantity must be an integer")
    if item['quantity'] < 1:
        raise ValueError("Quantity must be at least 1")
    return True


def calc_disc(customer_type, amount):
    """Calculate discount based on customer loyalty tier."""
    discount_rates = {
        'platinum': 0.20,
        'gold': 0.15,
        'silver': 0.10,
        'bronze': 0.05
    }
    rate = discount_rates.get(customer_type, 0)
    return round(amount * rate, 2)
EOF

cat > api_utils.js << 'EOF'
/**
 * API utility functions
 */

function getData(url) {
    return fetch(url)
        .then(response => {
            if (!response.ok) {
                throw new Error(`Request failed: ${response.status}`);
            }
            return response.json();
        })
        .then(data => ({
            success: true,
            payload: data,
            fetchedAt: new Date().toISOString()
        }))
        .catch(error => ({
            success: false,
            error: error.message,
            fetchedAt: new Date().toISOString()
        }));
}


function postData(url, body) {
    return fetch(url, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Accept': 'application/json'
        },
        body: JSON.stringify(body)
    })
        .then(response => response.json())
        .then(data => ({
            success: true,
            payload: data
        }));
}


function fmtDate(timestamp) {
    const date = new Date(timestamp);
    const options = {
        year: 'numeric',
        month: 'long',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit'
    };
    return date.toLocaleDateString('en-US', options);
}
EOF

git add .
git commit -m "Initial commit with legacy function names"

echo "Created base commit"

# FEATURE BRANCH: Developer renames functions to follow new naming convention
git checkout -b feature/naming-convention

cat > cart.py << 'EOF'
"""Shopping cart module."""

def calculate_order_total(items):
    """Calculate the total price including tax and shipping."""
    subtotal = sum(item['price'] * item['quantity'] for item in items)
    tax_rate = 0.08
    tax = subtotal * tax_rate
    shipping = 5.99 if subtotal < 50 else 0
    total = subtotal + tax + shipping
    return round(total, 2)


def format_currency(amount):
    """Format a number as a currency string."""
    if amount < 0:
        return f"-${abs(amount):,.2f}"
    return f"${amount:,.2f}"


def validate_cart_item(item):
    """Check if a cart item is valid."""
    required_fields = ['name', 'price', 'quantity']
    for field in required_fields:
        if field not in item:
            raise ValueError(f"Missing required field: {field}")
    if not isinstance(item['price'], (int, float)):
        raise TypeError("Price must be a number")
    if item['price'] < 0:
        raise ValueError("Price cannot be negative")
    if not isinstance(item['quantity'], int):
        raise TypeError("Quantity must be an integer")
    if item['quantity'] < 1:
        raise ValueError("Quantity must be at least 1")
    return True


def calculate_loyalty_discount(customer_type, amount):
    """Calculate discount based on customer loyalty tier."""
    discount_rates = {
        'platinum': 0.20,
        'gold': 0.15,
        'silver': 0.10,
        'bronze': 0.05
    }
    rate = discount_rates.get(customer_type, 0)
    return round(amount * rate, 2)
EOF

cat > api_utils.js << 'EOF'
/**
 * API utility functions
 */

function fetchJsonData(url) {
    return fetch(url)
        .then(response => {
            if (!response.ok) {
                throw new Error(`Request failed: ${response.status}`);
            }
            return response.json();
        })
        .then(data => ({
            success: true,
            payload: data,
            fetchedAt: new Date().toISOString()
        }))
        .catch(error => ({
            success: false,
            error: error.message,
            fetchedAt: new Date().toISOString()
        }));
}


function submitJsonData(url, body) {
    return fetch(url, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Accept': 'application/json'
        },
        body: JSON.stringify(body)
    })
        .then(response => response.json())
        .then(data => ({
            success: true,
            payload: data
        }));
}


function formatTimestamp(timestamp) {
    const date = new Date(timestamp);
    const options = {
        year: 'numeric',
        month: 'long',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit'
    };
    return date.toLocaleDateString('en-US', options);
}
EOF

git add .
git commit -m "Rename all functions to follow naming convention"

echo "Created feature branch with renamed functions"

# MAIN BRANCH: Tech lead marks old functions as deprecated and removes them
git checkout main

cat > cart.py << 'EOF'
"""Shopping cart module."""

# Legacy functions removed - see cart_v2.py for new implementations
EOF

cat > api_utils.js << 'EOF'
/**
 * API utility functions
 *
 * Legacy functions removed - migrated to new API client
 */
EOF

git add .
git commit -m "Remove deprecated legacy functions"

echo "Created main branch with removed functions"

echo ""
echo "============================================"
echo "Demo repository created at: $DEMO_DIR"
echo ""
echo "SCENARIO:"
echo "  - Feature branch: Renamed all functions to new convention"
echo "  - Main branch: Deleted all 'deprecated' functions"
echo ""
echo "WITHOUT move detection: Massive delete/add conflicts"
echo "WITH move detection: Auto-merges everything!"
echo ""
echo "Functions being matched:"
echo "  calc_total    →  calculate_order_total"
echo "  fmt_money     →  format_currency"
echo "  chk_item      →  validate_cart_item"
echo "  calc_disc     →  calculate_loyalty_discount"
echo "  getData       →  fetchJsonData"
echo "  postData      →  submitJsonData"
echo "  fmtDate       →  formatTimestamp"
echo ""
echo "To test:"
echo "  cd $DEMO_DIR"
echo "  g2 merge feature/naming-convention"
echo "============================================"
