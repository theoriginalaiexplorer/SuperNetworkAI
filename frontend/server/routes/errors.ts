import { Hono } from "hono";

export const errorRoutes = new Hono();

errorRoutes.get("/404", (c) => {
  return c.html(`
    <!DOCTYPE html>
<html lang="en" data-bs-theme="light">
    <head>
      <meta charset="UTF-8" />
      <meta name="viewport" content="width=device-width, initial-scale=1.0" />
      <title>404 — Not Found — SuperNetworkAI</title>
      <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.3/dist/css/bootstrap.min.css" integrity="sha384-QWTKZyjpPEjISv5WaRU9OFeRpok6YctnYmDr5pNlyT2bRjXh0JMhjY6hW+ALEwIH" crossorigin="anonymous" />
    </head>
    <body>
      <nav class="navbar navbar-expand-lg bg-body-tertiary">
        <div class="container">
          <a class="navbar-brand fw-bold" href="/">SuperNetworkAI</a>
          <div class="ms-auto d-flex gap-2">
            <a href="/login" class="nav-link">Log in</a>
            <a href="/signup" class="nav-link">Sign up</a>
          </div>
        </div>
      </nav>
      <main class="container d-flex justify-content-center align-items-center min-vh-100">
        <div class="card text-center" style="max-width: 500px;">
          <div class="card-body py-5">
            <h1 class="h4 mb-3">404</h1>
            <p class="text-muted mb-3">The page you're looking for doesn't exist or has been moved.</p>
            <p><a href="/dashboard" class="btn btn-primary">Return to Dashboard</a></p>
          </div>
        </div>
      </main>
    </body>
    </html>
  `);
});

errorRoutes.get("/500", (c) => {
  return c.html(`
    <!DOCTYPE html>
<html lang="en" data-bs-theme="light">
    <head>
      <meta charset="UTF-8" />
      <meta name="viewport" content="width=device-width, initial-scale=1.0" />
      <title>500 — Server Error — SuperNetworkAI</title>
      <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.3/dist/css/bootstrap.min.css" integrity="sha384-QWTKZyjpPEjISv5WaRU9OFeRpok6YctnYmDr5pNlyT2bRjXh0JMhjY6hW+ALEwIH" crossorigin="anonymous" />
    </head>
    <body>
      <nav class="navbar navbar-expand-lg bg-body-tertiary">
        <div class="container">
          <a class="navbar-brand fw-bold" href="/">SuperNetworkAI</a>
          <div class="ms-auto d-flex gap-2">
            <a href="/login" class="nav-link">Log in</a>
            <a href="/signup" class="nav-link">Sign up</a>
          </div>
        </div>
      </nav>
      <main class="container d-flex justify-content-center align-items-center min-vh-100">
        <div class="card text-center" style="max-width: 500px;">
          <div class="card-body py-5">
            <h1 class="h4 mb-3">500</h1>
            <p class="text-muted mb-3">Something went wrong on our end. Our team has been notified and is working to fix the issue.</p>
            <p><a href="/dashboard" class="btn btn-primary">Return to Dashboard</a></p>
          </div>
        </div>
      </main>
    </body>
    </html>
  `);
});

errorRoutes.get("/error", (c) => {
  const status = c.req.query("status") || "500";
  const message = c.req.query("message") || "Something went wrong";
  const title = status === "404" ? "Not Found" : "Server Error";

  return c.html(`
    <!DOCTYPE html>
<html lang="en" data-bs-theme="light">
    <head>
      <meta charset="UTF-8" />
      <meta name="viewport" content="width=device-width, initial-scale=1.0" />
      <title>${title} — SuperNetworkAI</title>
      <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.3/dist/css/bootstrap.min.css" integrity="sha384-QWTKZyjpPEjISv5WaRU9OFeRpok6YctnYmDr5pNlyT2bRjXh0JMhjY6hW+ALEwIH" crossorigin="anonymous" />
    </head>
    <body>
      <nav class="navbar navbar-expand-lg bg-body-tertiary">
        <div class="container">
          <a class="navbar-brand fw-bold" href="/">SuperNetworkAI</a>
          <div class="ms-auto d-flex gap-2">
            <a href="/login" class="nav-link">Log in</a>
            <a href="/signup" class="nav-link">Sign up</a>
          </div>
        </div>
      </nav>
      <main class="container d-flex justify-content-center align-items-center min-vh-100">
        <div class="card text-center" style="max-width: 500px;">
          <div class="card-body py-5">
            <h1 class="h4 mb-3">${status}</h1>
            <p class="text-muted mb-3">${message}</p>
            <p><a href="/dashboard" class="btn btn-primary">Return to Dashboard</a></p>
          </div>
        </div>
      </main>
    </body>
    </html>
  `);
});
