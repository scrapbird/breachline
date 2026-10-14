// Stripe Payment Links Integration for BreachLine
// This script handles purchase button clicks and redirects to Stripe Payment Links

// Payment Links are created per environment by Terraform (infra/stripe) and
// injected into the page as window.BREACHLINE_PAYMENT_LINKS ({monthly, yearly})
// by `make build-website`, so dev builds use the sandbox and prod the live links.
const PAYMENT_LINKS = window.BREACHLINE_PAYMENT_LINKS || {};

// Handle purchase button clicks
function handlePurchaseClick(plan) {
    const paymentLink = PAYMENT_LINKS[plan];
    
    if (!paymentLink) {
        console.error('Payment link not configured for plan:', plan);
        alert('Payment configuration error. Please contact support.');
        return;
    }
    
    // Redirect to Stripe Payment Link
    window.location.href = paymentLink;
}

// Set up event listeners when DOM is ready
document.addEventListener('DOMContentLoaded', function() {
    // Add click handlers to all purchase buttons
    const purchaseButtons = document.querySelectorAll('.purchase-btn[data-plan]');
    purchaseButtons.forEach(button => {
        button.addEventListener('click', function(e) {
            e.preventDefault();
            const plan = this.getAttribute('data-plan');
            handlePurchaseClick(plan);
        });
    });
});
