// Redirects to the login/setup page if this browser doesn't hold a valid
// dashboard session. Included on every page except login.html and
// initial.html.
//
// wire-pod's own first-run wizard (initial.html) has to run before an
// admin password means anything, so while pastInitialSetup is still
// false this script does nothing -- main.js's existing redirect to
// /initial.html handles that case.
(function () {
  fetch("/api/auth/status", { credentials: "same-origin" })
    .then(function (res) {
      return res.json();
    })
    .then(function (status) {
      if (!status.pastInitialSetup) {
        return;
      }
      if (!status.initialized || !status.authenticated) {
        window.location.replace("/login.html");
      }
    })
    .catch(function () {
      // /api/auth/status is always reachable regardless of setup state,
      // so a failure here is a real error, not a gating issue. Fail closed.
      window.location.replace("/login.html");
    });
})();
