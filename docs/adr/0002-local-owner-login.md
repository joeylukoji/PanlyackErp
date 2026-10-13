# ADR 0002: Local owner access

The attached login design informed a templ login page. The ERP keeps server-rendered pages and HTMX interactions; React, shadcn and client-side credential state would split the selected frontend stack.

The local MVP has one seeded organization. An explicit CLI creates or rotates its owner password. Users are separate from organization memberships, so each private request checks a session linked to a current membership and owner role. The application scopes every business operation to that seeded organization. The audit actor is now the authenticated user ID.

Passwords use a random salt and Argon2id with fixed parameters. The browser receives a random opaque HttpOnly SameSite cookie; PostgreSQL holds only its SHA-256 digest and an eight-hour expiry. Logout and password rotation revoke sessions. CSRF protection remains on all POST requests, including login and logout. Health and static assets stay public.

This is still a loopback-only demonstration. HTTP on localhost cannot provide a Secure cookie. Production mode remains disabled until TLS, rate limiting, invitation/recovery processes, role policy and operational hardening exist. The password bootstrap command never prints the password.
