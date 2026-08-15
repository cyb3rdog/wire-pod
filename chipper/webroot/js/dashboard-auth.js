// Redirects to the login/setup page if this browser doesn't hold a valid
// dashboard session. Included on every page except login.html; also
// covers initial.html, which needs this for a case past first-run: its
// connection-method form (escape pod/IP/custom host) doubles as a
// reconfiguration tool on an already-set-up server, and those
// /api-chipper/* calls require a session once pastInitialSetup is true
// (see dashboardauth.isProtected), same as everywhere else.
//
// wire-pod's own first-run wizard (initial.html) has to run before an
// admin password means anything, so while pastInitialSetup is still
// false this script does nothing on any page -- main.js's existing
// redirect to /initial.html handles that case.
(function () {
  var here = window.location.pathname + window.location.search;
  fetch("/api/auth/status", { credentials: "same-origin" })
    .then(function (res) {
      return res.json();
    })
    .then(function (status) {
      if (!status.pastInitialSetup) {
        return;
      }
      if (!status.initialized || !status.authenticated) {
        window.location.replace("/login.html?redirect=" + encodeURIComponent(here));
      }
    })
    .catch(function () {
      // /api/auth/status is always reachable regardless of setup state,
      // so a failure here is a real error, not a gating issue. Fail closed.
      window.location.replace("/login.html?redirect=" + encodeURIComponent(here));
    });
})();
