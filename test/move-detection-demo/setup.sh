#!/usr/bin/env bash
# Setup script for move detection demo
# This creates a scenario where BOTH branches rename the same function

set -e

DEMO_DIR="$(dirname "$0")/repo"

# Clean up any existing demo
rm -rf "$DEMO_DIR"
mkdir -p "$DEMO_DIR"
cd "$DEMO_DIR"

echo "=== Setting up move detection demo ==="
echo ""

# Initialize git repo
git init
git config user.email "demo@example.com"
git config user.name "Demo User"

# Create initial Python file
cat > utils.py << 'EOF'
"""Utility functions for data processing."""

def calculate_total(items):
    """Calculate total price of items with tax."""
    subtotal = sum(item['price'] * item['quantity'] for item in items)
    tax = subtotal * 0.08
    shipping = 5.99 if subtotal < 50 else 0
    return round(subtotal + tax + shipping, 2)


def format_price(amount):
    """Format amount as USD price."""
    return f"${amount:.2f}"


def validate_item(item):
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
EOF

# Create initial JavaScript file
cat > helpers.js << 'EOF'
/**
 * Helper functions
 */

function fetchUserData(userId) {
    return fetch(`/api/users/${userId}`)
        .then(response => response.json())
        .then(data => {
            return {
                id: data.id,
                name: data.name,
                email: data.email,
                createdAt: new Date(data.created_at)
            };
        });
}

function formatDate(date) {
    const options = { year: 'numeric', month: 'long', day: 'numeric' };
    return date.toLocaleDateString('en-US', options);
}

function debounce(func, wait) {
    let timeout;
    return function(...args) {
        clearTimeout(timeout);
        timeout = setTimeout(() => func.apply(this, args), wait);
    };
}
EOF

git add .
git commit -m "Initial commit"

echo "Created initial commit"

# Create feature branch - RENAME calculate_total to compute_order_total
git checkout -b feature/rename-functions

cat > utils.py << 'EOF'
"""Utility functions for data processing."""

def compute_order_total(items):
    """Calculate total price of items with tax."""
    subtotal = sum(item['price'] * item['quantity'] for item in items)
    tax = subtotal * 0.08
    shipping = 5.99 if subtotal < 50 else 0
    return round(subtotal + tax + shipping, 2)


def format_price(amount):
    """Format amount as USD price."""
    return f"${amount:.2f}"


def validate_item(item):
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
EOF

# Also rename fetchUserData to getUserById
cat > helpers.js << 'EOF'
/**
 * Helper functions
 */

function getUserById(userId) {
    return fetch(`/api/users/${userId}`)
        .then(response => response.json())
        .then(data => {
            return {
                id: data.id,
                name: data.name,
                email: data.email,
                createdAt: new Date(data.created_at)
            };
        });
}

function formatDate(date) {
    const options = { year: 'numeric', month: 'long', day: 'numeric' };
    return date.toLocaleDateString('en-US', options);
}

function debounce(func, wait) {
    let timeout;
    return function(...args) {
        clearTimeout(timeout);
        timeout = setTimeout(() => func.apply(this, args), wait);
    };
}
EOF

git add .
git commit -m "Rename calculate_total -> compute_order_total, fetchUserData -> getUserById"

echo "Created feature branch with renames"

# Go back to main - ALSO rename the same functions but to DIFFERENT names
git checkout main

cat > utils.py << 'EOF'
"""Utility functions for data processing."""

def get_cart_total(items):
    """Calculate total price of items with tax."""
    subtotal = sum(item['price'] * item['quantity'] for item in items)
    tax = subtotal * 0.08
    shipping = 5.99 if subtotal < 50 else 0
    return round(subtotal + tax + shipping, 2)


def format_price(amount):
    """Format amount as USD price."""
    return f"${amount:.2f}"


def validate_item(item):
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
EOF

# Also rename fetchUserData to loadUser
cat > helpers.js << 'EOF'
/**
 * Helper functions
 */

function loadUser(userId) {
    return fetch(`/api/users/${userId}`)
        .then(response => response.json())
        .then(data => {
            return {
                id: data.id,
                name: data.name,
                email: data.email,
                createdAt: new Date(data.created_at)
            };
        });
}

function formatDate(date) {
    const options = { year: 'numeric', month: 'long', day: 'numeric' };
    return date.toLocaleDateString('en-US', options);
}

function debounce(func, wait) {
    let timeout;
    return function(...args) {
        clearTimeout(timeout);
        timeout = setTimeout(() => func.apply(this, args), wait);
    };
}
EOF

git add .
git commit -m "Rename calculate_total -> get_cart_total, fetchUserData -> loadUser"

echo "Created conflicting renames on main"

echo ""
echo "=========================================="
echo "Demo repository created at: $DEMO_DIR"
echo ""
echo "Branches ready:"
echo "  main:                   has renamed functions"
echo "  feature/rename-functions: has different renames"
echo ""
echo "To test:"
echo "  cd $DEMO_DIR"
echo "  g2 merge feature/rename-functions"
echo "=========================================="
