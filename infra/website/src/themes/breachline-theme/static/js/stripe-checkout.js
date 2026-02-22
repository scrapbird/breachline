// Stripe Payment Links Integration for BreachLine
// This script handles purchase button clicks and redirects to Stripe Payment Links

// IMPORTANT: Create these Payment Links in your Stripe Dashboard
// Go to: Stripe Dashboard > Payment Links > Create payment link
// Then replace the URLs below with your actual Payment Link URLs
const PAYMENT_LINKS = {
    monthly: 'https://buy.stripe.com/test_4gM6oG0zJ29q4QD91c1gs00', // Replace with actual Payment Link
    yearly: 'https://buy.stripe.com/YOUR_YEARLY_LINK'    // Replace with actual Payment Link
};

// Handle purchase button clicks
function handlePurchaseClick(plan) {
    const paymentLink = PAYMENT_LINKS[plan];
    
    if (!paymentLink || paymentLink.includes('YOUR_')) {
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
