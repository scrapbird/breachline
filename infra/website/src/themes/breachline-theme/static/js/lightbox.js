// Home-page screenshot lightbox: open the hero image in a modal dialog.
document.addEventListener('DOMContentLoaded', function () {
    var dialog = document.getElementById('screenshot-dialog');
    if (!dialog || typeof dialog.showModal !== 'function') {
        return;
    }

    var trigger = document.querySelector('.hero-image-trigger');
    var closeBtn = dialog.querySelector('.image-dialog-close');

    if (trigger) {
        trigger.addEventListener('click', function () {
            dialog.showModal();
        });
    }

    if (closeBtn) {
        closeBtn.addEventListener('click', function () {
            dialog.close();
        });
    }

    // Click on the backdrop (outside the image) closes the dialog.
    dialog.addEventListener('click', function (event) {
        if (event.target === dialog) {
            dialog.close();
        }
    });
});
