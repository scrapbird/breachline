// Post-image lightbox: click any image in a blog post to open it enlarged in a
// modal dialog, matching the home-page screenshot zoom.
document.addEventListener('DOMContentLoaded', function () {
    var imgs = document.querySelectorAll('.post img');
    if (!imgs.length) {
        return;
    }

    var dialog = document.createElement('dialog');
    dialog.className = 'image-dialog';
    if (typeof dialog.showModal !== 'function') {
        return; // No <dialog> support: leave images as-is.
    }

    var closeBtn = document.createElement('button');
    closeBtn.type = 'button';
    closeBtn.className = 'image-dialog-close';
    closeBtn.setAttribute('aria-label', 'Close');
    closeBtn.innerHTML = '&times;';

    var big = document.createElement('img');

    dialog.appendChild(closeBtn);
    dialog.appendChild(big);
    document.body.appendChild(dialog);

    imgs.forEach(function (img) {
        img.style.cursor = 'zoom-in';
        img.addEventListener('click', function () {
            big.src = img.currentSrc || img.src;
            big.alt = img.alt || '';
            dialog.showModal();
        });
    });

    closeBtn.addEventListener('click', function () {
        dialog.close();
    });

    // Click on the backdrop (outside the image) closes the dialog.
    dialog.addEventListener('click', function (event) {
        if (event.target === dialog) {
            dialog.close();
        }
    });
});
