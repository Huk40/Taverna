// Run only against a disposable deployment with an empty database.
import assert from "node:assert/strict";

const origin = process.env.TEST_API_URL ?? "http://localhost:5173";
assert.ok(process.env.TEST_ADMIN_EMAIL && process.env.TEST_ADMIN_PASSWORD, "Set test account credentials");
const page = await fetch(origin);
assert.equal(page.status, 200);
const html = await page.text();
assert.ok(html.includes("/assets/") && !html.includes("/@vite/client"), "Expected a built frontend");
for (const route of ["/admin", "/health", "/api/catalog"]) {
  assert.equal((await fetch(origin + route)).status, 200, route);
}
const registration = await fetch(origin + "/api/auth/register", {
  method: "POST",
  headers: { "Content-Type": "application/json", "X-Taverna-Request": "1", Origin: origin },
  body: JSON.stringify({ email: process.env.TEST_ADMIN_EMAIL, password: process.env.TEST_ADMIN_PASSWORD }),
});
assert.equal(registration.status, 201, "Registration through frontend proxy failed");
assert.ok(registration.headers.get("X-Request-ID"));
assert.ok(registration.headers.getSetCookie().some((cookie) => cookie.includes("HttpOnly") && cookie.includes("SameSite=Lax")));
assert.equal((await registration.json()).user.role, "user");
console.log("PASS: built frontend, SPA routing, proxied API, same-origin registration and session cookie");
