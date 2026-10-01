import { describe, expect, it } from "vitest";

import indexHTML from "../index.html?raw";

describe("content security policy", () => {
  it("allows the deployed management API origin", () => {
    const document = new DOMParser().parseFromString(indexHTML, "text/html");
    const policy = document
      .querySelector('meta[http-equiv="Content-Security-Policy"]')
      ?.getAttribute("content");
    const connectSources = policy
      ?.split(";")
      .find((directive) => directive.trim().startsWith("connect-src "))
      ?.trim()
      .split(/\s+/)
      .slice(1);

    expect(connectSources).toEqual(["https://ts-management-api.diogo-nb.com.br"]);
  });
});
