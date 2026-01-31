---
title: "Purchase Successful"
date: 2025-01-01T00:00:00Z
draft: false
---

# Thank You for Your Purchase!

Your BreachLine Premium license is being processed.

## What's Next?

1. **Check Your Email** - You'll receive your license key within a few minutes at the email address you provided during checkout.

2. **Download BreachLine** - If you haven't already, [download the application here](/download/).

3. **Activate Your License** - Open BreachLine and go to Help → Enter License Key, then paste your license key.

## Need Help?

If you don't receive your license key within 10 minutes, please check your spam folder or contact us at support@breachline.example.com with your order details.

---

<div id="order-details" style="display:none;">
    <h3>Order Details</h3>
    <p><strong>Order ID:</strong> <span id="session-id"></span></p>
</div>

<script>
// Extract session ID from URL if present
const urlParams = new URLSearchParams(window.location.search);
const sessionId = urlParams.get('session_id');

if (sessionId) {
    document.getElementById('order-details').style.display = 'block';
    document.getElementById('session-id').textContent = sessionId;
}
</script>
