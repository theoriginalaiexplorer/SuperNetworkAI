import { Hono } from "hono";
import { initializeApp, getApps } from "firebase/app";
import {
  getAuth,
  signInWithEmailAndPassword,
  createUserWithEmailAndPassword,
  sendEmailVerification,
  applyActionCode,
} from "firebase/auth";
import { SignJWT } from "jose";
import { setSessionCookies, clearSessionCookies } from "../middleware/session";
import { apiClient } from "../lib/api";

const firebaseConfig = {
  apiKey:     process.env.FIREBASE_API_KEY!,
  authDomain: process.env.FIREBASE_AUTH_DOMAIN!,
  projectId:  process.env.FIREBASE_PROJECT_ID!,
};

// Initialize once — safe under hot reload
const firebaseApp = getApps().length ? getApps()[0] : initializeApp(firebaseConfig);
const firebaseAuth = getAuth(firebaseApp);

const BFF_JWT_SECRET = new TextEncoder().encode(process.env.BFF_JWT_SECRET!);

// UUIDv5 namespace for SuperNetworkAI (DNS namespace: 6ba7b810-9dad-11d1-80b4-00c04fd430c8)
const SN_NS = "6ba7b810-9dad-11d1-80b4-00c04fd430c8";

function hexToBytes(hex: string): Uint8Array {
  const clean = hex.replace(/-/g, "");
  const bytes = new Uint8Array(clean.length / 2);
  for (let i = 0; i < bytes.length; i++) {
    bytes[i] = parseInt(clean.slice(i * 2, i * 2 + 2), 16);
  }
  return bytes;
}

// firebaseUidToUuid derives a deterministic UUIDv5 from a Firebase UID.
// Stable across sessions — same Firebase UID always produces the same UUID.
async function firebaseUidToUuid(firebaseUid: string): Promise<string> {
  const nsBytes = hexToBytes(SN_NS);
  const nameBytes = new TextEncoder().encode(firebaseUid);
  const input = new Uint8Array(nsBytes.length + nameBytes.length);
  input.set(nsBytes);
  input.set(nameBytes, nsBytes.length);

  const hashBuf = await crypto.subtle.digest("SHA-1", input);
  const hash = new Uint8Array(hashBuf);

  // Set version 5 (0101) in the high nibble of byte 6
  hash[6] = (hash[6] & 0x0f) | 0x50;
  // Set variant bits (10xx) in byte 8
  hash[8] = (hash[8] & 0x3f) | 0x80;

  const hex = Array.from(hash.slice(0, 16))
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");

  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20, 32)}`;
}

// signBffJwt issues a 1-hour HS256 JWT with the user's stable UUID as `sub`
// and their email as an additional claim so the Go API can upsert the user row.
async function signBffJwt(userUuid: string, email: string): Promise<string> {
  return new SignJWT({ sub: userUuid, email })
    .setProtectedHeader({ alg: "HS256" })
    .setIssuedAt()
    .setExpirationTime("1h")
    .sign(BFF_JWT_SECRET);
}

export const authRoutes = new Hono();

// POST /auth/login
authRoutes.post("/login", async (c) => {
  const body = await c.req.parseBody();
  const email = String(body.email ?? "");
  const password = String(body.password ?? "");

  console.log(`[AUTH] Login attempt for: ${email}`);

  try {
    const cred = await signInWithEmailAndPassword(firebaseAuth, email, password);
    console.log(`[AUTH] Login success for: ${email}, UID: ${cred.user.uid}`);
    const userUuid = await firebaseUidToUuid(cred.user.uid);
    const bffJwt = await signBffJwt(userUuid, cred.user.email ?? "");
    setSessionCookies(c, { accessToken: bffJwt, refreshToken: cred.user.refreshToken });
    // Ensure user + profile rows exist in DB before any onboarding calls
    await apiClient(bffJwt).get("/api/v1/users/me").catch(() => {});
    // HTMX request: use HX-Redirect; plain form POST: use 303 redirect
    if (c.req.header("HX-Request")) {
      c.header("HX-Redirect", "/dashboard");
      return c.body(null, 204);
    }
    return c.redirect("/dashboard", 303);
  } catch (err: any) {
    console.error(`[AUTH] Login failed for: ${email}`, err.code, err.message);
    if (c.req.header("HX-Request")) {
      return c.html(`<div id="auth-error" class="alert alert-danger">Invalid email or password</div>`);
    }
    return c.redirect("/login?error=invalid_credentials", 303);
  }
});

// POST /auth/signup
authRoutes.post("/signup", async (c) => {
  const body = await c.req.parseBody();
  const email = String(body.email ?? "");
  const password = String(body.password ?? "");

  console.log(`[AUTH] Signup attempt for: ${email}`);

  try {
    const cred = await createUserWithEmailAndPassword(firebaseAuth, email, password);
    console.log(`[AUTH] Signup success for: ${email}, UID: ${cred.user.uid}`);
    await sendEmailVerification(cred.user);
    const userUuid = await firebaseUidToUuid(cred.user.uid);
    const bffJwt = await signBffJwt(userUuid, cred.user.email ?? "");
    setSessionCookies(c, { accessToken: bffJwt, refreshToken: cred.user.refreshToken });
    // Ensure user + profile rows exist in DB before onboarding steps write to them
    await apiClient(bffJwt).get("/api/v1/users/me").catch(() => {});
    // HTMX request: use HX-Redirect; plain form POST: use 303 redirect
    if (c.req.header("HX-Request")) {
      c.header("HX-Redirect", "/onboarding/step1");
      return c.body(null, 204);
    }
    return c.redirect("/onboarding/step1", 303);
  } catch (err: any) {
    console.error(`[AUTH] Signup failed for: ${email}`, err.code, err.message);
    const msg = err.code === "auth/email-already-in-use"
      ? "An account with this email already exists. <a href=\"/login\">Log in instead?</a>"
      : "Signup failed. Please try again.";
    if (c.req.header("HX-Request")) {
      return c.html(`<div id="auth-error" class="alert alert-danger">${msg}</div>`);
    }
    return c.redirect("/signup?error=signup_failed", 303);
  }
});

// GET /auth/logout
authRoutes.get("/logout", (c) => {
  clearSessionCookies(c);
  return c.redirect("/");
});

// GET /auth/confirm — applies a Firebase email verification action code
// Firebase sends ?oobCode=... in the verification email link
authRoutes.get("/confirm", async (c) => {
  const oobCode = c.req.query("oobCode") ?? "";
  try {
    await applyActionCode(firebaseAuth, oobCode);
    return c.redirect("/login?verified=1");
  } catch {
    return c.redirect("/login?error=invalid_confirmation_link");
  }
});
