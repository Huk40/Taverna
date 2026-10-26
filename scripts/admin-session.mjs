import assert from "node:assert/strict";

// Credentials are supplied by the operator; never save them in source control.
export async function adminSession(origin) {
  const email = process.env.TEST_ADMIN_EMAIL;
  const password = process.env.TEST_ADMIN_PASSWORD;
  assert.ok(email && password, "Set TEST_ADMIN_EMAIL and TEST_ADMIN_PASSWORD for an existing administrator");
  const response = await fetch(origin + "/api/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json", "X-Taverna-Request": "1" },
    body: JSON.stringify({ email, password }),
  });
  assert.equal(response.status, 200, "Administrator login failed");
  assert.equal((await response.json()).user.role, "admin", "Account must have the admin role");
  const cookie = response.headers.getSetCookie().find((value) => value.startsWith("taverna_session="))?.split(";")[0];
  assert.ok(cookie, "Session cookie is missing");
  return { Cookie: cookie, "X-Taverna-Request": "1" };
}
