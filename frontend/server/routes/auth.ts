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

const firebaseConfig = {
  apiKey:     process.env.FIREBASE_API_KEY!,
  authDomain: process.env.FIREBASE_AUTH_DOMAIN!,
  projectId:  process.env.FIREBASE_PROJECT_ID!,
};

// Initialize once — safe under hot reload
const firebaseApp = getApps().length ? getApps()[0] : initializeApp(firebaseConfig);
const firebaseAuth = getAuth(firebaseApp);

const BFF_JWT_SECRET = new TextEncoder().encode(process.env.BFF_JWT_SECRET!);

// signBffJwt issues a 1-hour HS256 JWT with the user's stable UUID as `sub`.
// The Go API validates this JWT using the same BFF_JWT_SECRET.
async function signBffJwt(userUuid: string): Promise<string> {
  return new SignJWT({ sub: userUuid })
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

  try {
    const cred = await signInWithEmailAndPassword(firebaseAuth, email, password);
    // NOTE: UUID is generated fresh per login session.
    // Post-emergency: add auth_mapping table (firebase_uid → uuid) for stable UUIDs.
    const userUuid = crypto.randomUUID();
    const bffJwt = await signBffJwt(userUuid);
    setSessionCookies(c, { accessToken: bffJwt, refreshToken: cred.user.refreshToken });
    c.header("HX-Redirect", "/dashboard");
    return c.body(null, 204);
  } catch {
    return c.html(
      `<div id="auth-error" class="alert alert-danger">Invalid email or password</div>`
    );
  }
});

// POST /auth/signup
authRoutes.post("/signup", async (c) => {
  const body = await c.req.parseBody();
  const email = String(body.email ?? "");
  const password = String(body.password ?? "");

  try {
    const cred = await createUserWithEmailAndPassword(firebaseAuth, email, password);
    await sendEmailVerification(cred.user);
    const userUuid = crypto.randomUUID();
    const bffJwt = await signBffJwt(userUuid);
    setSessionCookies(c, { accessToken: bffJwt, refreshToken: cred.user.refreshToken });
    c.header("HX-Redirect", "/onboarding/step1");
    return c.body(null, 204);
  } catch (err: any) {
    return c.html(
      `<div id="auth-error" class="alert alert-danger">${err.message ?? "Signup failed"}</div>`
    );
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
