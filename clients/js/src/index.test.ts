import { test } from "node:test";
import assert from "node:assert/strict";
import { SnagboxClient, SnagboxError, type Issue } from "./index.js";

function makeIssue(overrides: Partial<Issue> = {}): Issue {
  return {
    id: "issue-1",
    project_id: null,
    source: "test",
    text: "hello",
    meta: {},
    created_at: "2026-07-19T00:00:00Z",
    attachments: [],
    ...overrides,
  };
}

test("reportIssue: JSON path (no photos)", async () => {
  let capturedUrl: string | undefined;
  let capturedInit: RequestInit | undefined;
  let callCount = 0;

  const fakeFetch = (async (
    input: RequestInfo | URL,
    init?: RequestInit,
  ) => {
    callCount++;
    capturedUrl = String(input);
    capturedInit = init;
    return new Response(
      JSON.stringify(makeIssue({ text: "hello world", meta: { foo: "bar" } })),
      { status: 201, headers: { "Content-Type": "application/json" } },
    );
  }) as typeof fetch;

  const client = new SnagboxClient("https://snagbox.example.com/", "tkn", {
    fetch: fakeFetch,
  });

  const issue = await client.reportIssue({
    text: "hello world",
    meta: { foo: "bar" },
  });

  assert.equal(callCount, 1);
  assert.equal(capturedUrl, "https://snagbox.example.com/api/v1/issues");

  const headers = new Headers(capturedInit?.headers);
  assert.equal(headers.get("Content-Type"), "application/json");
  assert.equal(headers.get("Authorization"), "Bearer tkn");

  assert.equal(typeof capturedInit?.body, "string");
  const parsedBody = JSON.parse(capturedInit!.body as string);
  assert.equal(parsedBody.text, "hello world");
  assert.deepEqual(parsedBody.meta, { foo: "bar" });

  assert.equal(issue.text, "hello world");
  assert.deepEqual(issue.meta, { foo: "bar" });
});

test("reportIssue: multipart path (with photo)", async () => {
  let capturedInit: RequestInit | undefined;

  const fakeFetch = (async (
    _input: RequestInfo | URL,
    init?: RequestInit,
  ) => {
    capturedInit = init;
    return new Response(
      JSON.stringify(
        makeIssue({
          attachments: [
            {
              id: "att-1",
              filename: "photo.png",
              mime: "image/png",
              size_bytes: 3,
              url: "https://snagbox.example.com/files/att-1",
            },
          ],
        }),
      ),
      { status: 201, headers: { "Content-Type": "application/json" } },
    );
  }) as typeof fetch;

  const client = new SnagboxClient("https://snagbox.example.com", "tkn", {
    fetch: fakeFetch,
  });

  const issue = await client.reportIssue({
    text: "with photo",
    photos: [
      {
        filename: "photo.png",
        contentType: "image/png",
        data: new Uint8Array([1, 2, 3]),
      },
    ],
  });

  assert.ok(capturedInit?.body instanceof FormData);
  const form = capturedInit!.body as FormData;

  assert.equal(form.get("text"), "with photo");

  const photoEntry = form.get("photo");
  assert.ok(photoEntry instanceof Blob);
  // File extends Blob and carries a name in Node's FormData/Blob implementation.
  assert.equal((photoEntry as File).name, "photo.png");

  const headers = new Headers(capturedInit?.headers);
  assert.equal(headers.get("Authorization"), "Bearer tkn");
  // Content-Type must NOT be set manually for multipart requests.
  assert.equal(headers.get("Content-Type"), null);

  assert.equal(issue.attachments.length, 1);
  assert.equal(issue.attachments[0].filename, "photo.png");
});

test("reportIssue: error path rejects with SnagboxError", async () => {
  const fakeFetch = (async () => {
    return new Response(JSON.stringify({ error: "unauthorized" }), {
      status: 401,
    });
  }) as typeof fetch;

  const client = new SnagboxClient("https://snagbox.example.com", "bad-tkn", {
    fetch: fakeFetch,
  });

  await assert.rejects(
    () => client.reportIssue({ text: "hi" }),
    (err: unknown) => {
      assert.ok(err instanceof SnagboxError);
      const snagboxErr = err as SnagboxError;
      assert.equal(snagboxErr.status, 401);
      assert.equal(snagboxErr.message, "unauthorized");
      return true;
    },
  );
});

test("reportIssue: empty request rejects without calling fetch", async () => {
  let called = false;
  const fakeFetch = (async () => {
    called = true;
    return new Response("{}", { status: 201 });
  }) as typeof fetch;

  const client = new SnagboxClient("https://snagbox.example.com", "tkn", {
    fetch: fakeFetch,
  });

  await assert.rejects(
    () => client.reportIssue({}),
    /snagbox: empty issue/,
  );
  assert.equal(called, false);
});
