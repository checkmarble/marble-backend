import { expect, test } from "vitest";
import { isRole, Role, adaptRole } from "./Role";

test("isRole", () => {
  expect(isRole("VIEWER")).toBe(true);
  expect(isRole("BUILDER")).toBe(true);
  expect(isRole("PUBLISHER")).toBe(true);
  expect(isRole("ADMIN")).toBe(true);
  expect(isRole("API_CLIENT")).toBe(true);
  expect(isRole("MARBLE_ADMIN")).toBe(true);
});

test("isRole fail", () => {
  expect(isRole("NOT A ROLE")).toBe(false);
});

test("adaptRole", () => {
  expect(adaptRole("VIEWER")).toBe(Role.VIEWER);
});
