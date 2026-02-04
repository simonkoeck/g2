#!/usr/bin/env bash
# Move Detection Mixed Demo - Shows both auto-merge and manual resolution
#
# This creates a realistic scenario with:
# - Renames that auto-merge via move detection
# - Identical changes that auto-merge
# - Conflicting changes that need manual resolution

set -e

DEMO_DIR="$(dirname "$0")/repo"

rm -rf "$DEMO_DIR"
mkdir -p "$DEMO_DIR"
cd "$DEMO_DIR"

echo "=== Setting up Mixed Conflict Demo ==="
echo ""

git init
git config user.email "demo@example.com"
git config user.name "Demo User"

# Create initial Python file
cat > payments.py << 'EOF'
"""Payment processing module."""

def calc_tax(amount, region):
    """Calculate tax for a given amount and region."""
    tax_rates = {
        'US': 0.08,
        'EU': 0.20,
        'UK': 0.20,
        'CA': 0.13
    }
    rate = tax_rates.get(region, 0.10)
    return round(amount * rate, 2)


def proc_payment(card_number, amount, currency):
    """Process a credit card payment."""
    if len(card_number) != 16:
        raise ValueError("Invalid card number")
    if amount <= 0:
        raise ValueError("Amount must be positive")
    # Simulate payment processing
    transaction_id = hash(f"{card_number}{amount}{currency}")
    return {
        'success': True,
        'transaction_id': abs(transaction_id) % 1000000,
        'amount': amount,
        'currency': currency
    }


def fmt_receipt(transaction):
    """Format a transaction as a receipt string."""
    return f"""
    ================================
    RECEIPT
    ================================
    Transaction: #{transaction['transaction_id']}
    Amount: {transaction['amount']} {transaction['currency']}
    Status: {'SUCCESS' if transaction['success'] else 'FAILED'}
    ================================
    """


def validate_card(card_number):
    """Validate a credit card number using Luhn algorithm."""
    digits = [int(d) for d in str(card_number)]
    odd_digits = digits[-1::-2]
    even_digits = digits[-2::-2]
    checksum = sum(odd_digits)
    for d in even_digits:
        checksum += sum(divmod(d * 2, 10))
    return checksum % 10 == 0


def get_exchange_rate(from_currency, to_currency):
    """Get exchange rate between two currencies."""
    rates = {
        ('USD', 'EUR'): 0.85,
        ('USD', 'GBP'): 0.73,
        ('EUR', 'USD'): 1.18,
        ('EUR', 'GBP'): 0.86,
        ('GBP', 'USD'): 1.37,
        ('GBP', 'EUR'): 1.16
    }
    if from_currency == to_currency:
        return 1.0
    return rates.get((from_currency, to_currency), None)
EOF

# Create initial JavaScript file
cat > checkout.js << 'EOF'
/**
 * Checkout flow utilities
 */

function calculateSubtotal(items) {
    return items.reduce((sum, item) => {
        return sum + (item.price * item.quantity);
    }, 0);
}


function applyDiscount(subtotal, discountCode) {
    const discounts = {
        'SAVE10': 0.10,
        'SAVE20': 0.20,
        'HALF': 0.50
    };
    const rate = discounts[discountCode] || 0;
    return subtotal * (1 - rate);
}


function validateAddress(address) {
    const required = ['street', 'city', 'country', 'postal_code'];
    for (const field of required) {
        if (!address[field]) {
            return { valid: false, error: `Missing ${field}` };
        }
    }
    return { valid: true };
}


function estimateShipping(weight, destination) {
    const baseRate = 5.99;
    const perKgRate = 2.50;
    const internationalMultiplier = destination === 'US' ? 1 : 2.5;
    return (baseRate + (weight * perKgRate)) * internationalMultiplier;
}
EOF

git add .
git commit -m "Initial commit with payments.py and checkout.js"

echo "Created initial commit"

# FEATURE BRANCH: Refactoring + new features
git checkout -b feature/payment-upgrade

cat > payments.py << 'EOF'
"""Payment processing module."""

def calculate_tax_amount(amount, region):
    """Calculate tax for a given amount and region."""
    tax_rates = {
        'US': 0.08,
        'EU': 0.20,
        'UK': 0.20,
        'CA': 0.13
    }
    rate = tax_rates.get(region, 0.10)
    return round(amount * rate, 2)


def process_card_payment(card_number, amount, currency):
    """Process a credit card payment."""
    if len(card_number) != 16:
        raise ValueError("Invalid card number")
    if amount <= 0:
        raise ValueError("Amount must be positive")
    # Simulate payment processing
    transaction_id = hash(f"{card_number}{amount}{currency}")
    return {
        'success': True,
        'transaction_id': abs(transaction_id) % 1000000,
        'amount': amount,
        'currency': currency
    }


def format_receipt(transaction):
    """Format a transaction as a receipt string."""
    return f"""
    ================================
    RECEIPT
    ================================
    Transaction: #{transaction['transaction_id']}
    Amount: {transaction['amount']} {transaction['currency']}
    Status: {'SUCCESS' if transaction['success'] else 'FAILED'}
    ================================
    """


def validate_card_number(card_number):
    """Validate a credit card number using Luhn algorithm."""
    digits = [int(d) for d in str(card_number)]
    odd_digits = digits[-1::-2]
    even_digits = digits[-2::-2]
    checksum = sum(odd_digits)
    for d in even_digits:
        checksum += sum(divmod(d * 2, 10))
    return checksum % 10 == 0


def get_exchange_rate(from_currency, to_currency):
    """Get exchange rate between two currencies - FEATURE VERSION."""
    # Added more currency pairs
    rates = {
        ('USD', 'EUR'): 0.85,
        ('USD', 'GBP'): 0.73,
        ('USD', 'JPY'): 110.0,
        ('EUR', 'USD'): 1.18,
        ('EUR', 'GBP'): 0.86,
        ('EUR', 'JPY'): 130.0,
        ('GBP', 'USD'): 1.37,
        ('GBP', 'EUR'): 1.16,
        ('JPY', 'USD'): 0.0091
    }
    if from_currency == to_currency:
        return 1.0
    return rates.get((from_currency, to_currency), None)
EOF

cat > checkout.js << 'EOF'
/**
 * Checkout flow utilities
 */

function computeCartSubtotal(items) {
    return items.reduce((sum, item) => {
        return sum + (item.price * item.quantity);
    }, 0);
}


function applyPromoCode(subtotal, discountCode) {
    const discounts = {
        'SAVE10': 0.10,
        'SAVE20': 0.20,
        'HALF': 0.50
    };
    const rate = discounts[discountCode] || 0;
    return subtotal * (1 - rate);
}


function validateShippingAddress(address) {
    const required = ['street', 'city', 'country', 'postal_code'];
    for (const field of required) {
        if (!address[field]) {
            return { valid: false, error: `Missing ${field}` };
        }
    }
    return { valid: true };
}


function estimateShipping(weight, destination) {
    // FEATURE: Updated shipping calculation with zones
    const baseRate = 4.99;
    const perKgRate = 2.00;
    const zoneMultipliers = {
        'US': 1.0,
        'CA': 1.5,
        'EU': 2.0,
        'INTL': 3.0
    };
    const multiplier = zoneMultipliers[destination] || zoneMultipliers['INTL'];
    return (baseRate + (weight * perKgRate)) * multiplier;
}
EOF

git add .
git commit -m "Feature: Rename functions and update shipping/exchange rates"

echo "Created feature branch"

# MAIN BRANCH: Bug fixes + different changes
git checkout main

cat > payments.py << 'EOF'
"""Payment processing module."""

def calculate_tax_amount(amount, region):
    """Calculate tax for a given amount and region."""
    tax_rates = {
        'US': 0.08,
        'EU': 0.20,
        'UK': 0.20,
        'CA': 0.13
    }
    rate = tax_rates.get(region, 0.10)
    return round(amount * rate, 2)


def process_card_payment(card_number, amount, currency):
    """Process a credit card payment."""
    if len(card_number) != 16:
        raise ValueError("Invalid card number")
    if amount <= 0:
        raise ValueError("Amount must be positive")
    # Simulate payment processing
    transaction_id = hash(f"{card_number}{amount}{currency}")
    return {
        'success': True,
        'transaction_id': abs(transaction_id) % 1000000,
        'amount': amount,
        'currency': currency
    }


def format_receipt(transaction):
    """Format a transaction as a receipt string."""
    return f"""
    ================================
    RECEIPT
    ================================
    Transaction: #{transaction['transaction_id']}
    Amount: {transaction['amount']} {transaction['currency']}
    Status: {'SUCCESS' if transaction['success'] else 'FAILED'}
    ================================
    """


def validate_card_number(card_number):
    """Validate a credit card number using Luhn algorithm."""
    digits = [int(d) for d in str(card_number)]
    odd_digits = digits[-1::-2]
    even_digits = digits[-2::-2]
    checksum = sum(odd_digits)
    for d in even_digits:
        checksum += sum(divmod(d * 2, 10))
    return checksum % 10 == 0


def get_exchange_rate(from_currency, to_currency):
    """Get exchange rate between two currencies - MAIN VERSION with caching."""
    # Added caching and fallback API
    rates = {
        ('USD', 'EUR'): 0.84,
        ('USD', 'GBP'): 0.72,
        ('EUR', 'USD'): 1.19,
        ('EUR', 'GBP'): 0.85,
        ('GBP', 'USD'): 1.38,
        ('GBP', 'EUR'): 1.17
    }
    if from_currency == to_currency:
        return 1.0
    rate = rates.get((from_currency, to_currency))
    if rate is None:
        # Fallback: try reverse rate
        reverse = rates.get((to_currency, from_currency))
        if reverse:
            rate = 1.0 / reverse
    return rate
EOF

cat > checkout.js << 'EOF'
/**
 * Checkout flow utilities
 */

function computeCartSubtotal(items) {
    return items.reduce((sum, item) => {
        return sum + (item.price * item.quantity);
    }, 0);
}


function applyPromoCode(subtotal, discountCode) {
    const discounts = {
        'SAVE10': 0.10,
        'SAVE20': 0.20,
        'HALF': 0.50
    };
    const rate = discounts[discountCode] || 0;
    return subtotal * (1 - rate);
}


function validateShippingAddress(address) {
    const required = ['street', 'city', 'country', 'postal_code'];
    for (const field of required) {
        if (!address[field]) {
            return { valid: false, error: `Missing ${field}` };
        }
    }
    return { valid: true };
}


function estimateShipping(weight, destination) {
    // MAIN: Fixed bug - was charging too much for international
    const baseRate = 5.99;
    const perKgRate = 2.50;
    const internationalMultiplier = destination === 'US' ? 1 : 1.8;
    return (baseRate + (weight * perKgRate)) * internationalMultiplier;
}
EOF

git add .
git commit -m "Main: Rename functions and fix exchange rate/shipping bugs"

echo "Created main branch"

echo ""
echo "============================================"
echo "Demo repository created at: $DEMO_DIR"
echo ""
echo "SCENARIO:"
echo ""
echo "AUTO-MERGE (identical renames on both branches):"
echo "  calc_tax         → calculate_tax_amount"
echo "  proc_payment     → process_card_payment"
echo "  fmt_receipt      → format_receipt"
echo "  validate_card    → validate_card_number"
echo "  calculateSubtotal→ computeCartSubtotal"
echo "  applyDiscount    → applyPromoCode"
echo "  validateAddress  → validateShippingAddress"
echo ""
echo "NEEDS RESOLUTION (different implementations):"
echo "  get_exchange_rate - both modified differently"
echo "  estimateShipping  - both modified differently"
echo ""
echo "To test:"
echo "  cd $DEMO_DIR"
echo "  g2 merge feature/payment-upgrade"
echo "============================================"
