// Redirects to the login/setup page if this browser doesn't hold a valid
// dashboard session. Included on every page except login.html itself.
(function () {
  fetch("/api/auth/status", { credentials: "same-origin" })
    .then(function (res) {
      return res.json();
    })
    .then(function (status) {
      if (!status.initialized || !status.authenticated) {
        window.location.replace("/login.html");
      }
    })
    .catch(function () {
      // If we can't even check status, fail closed.
      window.location.replace("/login.html");
    });
})();
