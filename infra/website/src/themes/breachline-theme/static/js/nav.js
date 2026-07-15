// Mobile nav: toggle the hamburger menu open/closed.
document.addEventListener('DOMContentLoaded', function () {
    var toggle = document.querySelector('.nav-toggle');
    var menu = document.getElementById('nav-menu');
    if (!toggle || !menu) {
        return;
    }

    function setOpen(open) {
        menu.classList.toggle('open', open);
        toggle.classList.toggle('open', open);
        toggle.setAttribute('aria-expanded', open ? 'true' : 'false');
    }

    toggle.addEventListener('click', function () {
        setOpen(!menu.classList.contains('open'));
    });

    // Close after tapping a link (navigating to a new page/section).
    menu.addEventListener('click', function (event) {
        if (event.target.closest('a')) {
            setOpen(false);
        }
    });
});
